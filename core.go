package main

// This package is solely a subprocess adapter for the original Vault recovery
// executable. It contains no parsing of archives, decryption, or restore logic.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var storageTypes = map[string]bool{"local": true, "s3": true, "sftp": true, "smb": true, "webdav": true}

// Connection holds only paths, never passphrase text. The upstream CLI reads
// credentials from the user's own key / passphrase / config files.
type Connection struct {
	Type           string
	BackupFolder   string
	ConfigFile     string
	KeyFile        string
	PassphraseFile string
}

func absPath(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", errors.New("a path is required")
	}
	return filepath.Abs(strings.TrimSpace(s))
}

func (c Connection) Flags() ([]string, error) {
	kind := strings.ToLower(strings.TrimSpace(c.Type))
	if !storageTypes[kind] {
		return nil, fmt.Errorf("unsupported storage type %q", c.Type)
	}
	out := []string{"--type", kind}
	if kind == "local" {
		if c.ConfigFile != "" {
			return nil, errors.New("local storage uses the backup folder; clear remote settings JSON")
		}
		path, err := absPath(c.BackupFolder)
		if err != nil {
			return nil, errors.New("select the local backup folder")
		}
		st, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("backup folder: %w", err)
		}
		if !st.IsDir() {
			return nil, errors.New("backup folder must be a directory")
		}
		out = append(out, "--path", path)
	} else {
		if c.BackupFolder != "" {
			return nil, errors.New("remote storage uses the settings JSON; clear local backup folder")
		}
		path, err := absPath(c.ConfigFile)
		if err != nil {
			return nil, errors.New("select the remote settings JSON file")
		}
		out = append(out, "--config-file", path)
	}
	for _, spec := range [][2]string{{"--key", c.KeyFile}, {"--passphrase-file", c.PassphraseFile}} {
		if spec[1] != "" {
			p, err := absPath(spec[1])
			if err != nil {
				return nil, err
			}
			out = append(out, spec[0], p)
		}
	}
	return out, nil
}

// Resolve a path, including existing junctions/symlinks along its prefix. A
// destination is permitted not to exist yet. This is an extra GUI-side guard;
// the upstream engine is authoritative for filesystem extraction safety.
func canonicalPath(p string) (string, error) {
	full, err := absPath(p)
	if err != nil {
		return "", err
	}
	walk := full
	var components []string
	for {
		if _, e := os.Lstat(walk); e == nil {
			real, e := filepath.EvalSymlinks(walk)
			if e != nil {
				return "", e
			}
			for i := len(components) - 1; i >= 0; i-- {
				real = filepath.Join(real, components[i])
			}
			return filepath.Clean(real), nil
		} else if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(walk)
		if parent == walk {
			return "", fmt.Errorf("cannot resolve %q", full)
		}
		components = append(components, filepath.Base(walk))
		walk = parent
	}
}

func pathIsWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	} // different volumes on Windows
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func validateDestination(c Connection, dest string) error {
	target, err := canonicalPath(dest)
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	if st, e := os.Stat(target); e == nil && !st.IsDir() {
		return errors.New("restore destination is a file, not a folder")
	}
	if c.Type == "local" {
		source, err := canonicalPath(c.BackupFolder)
		if err != nil {
			return err
		}
		if pathIsWithin(source, target) || pathIsWithin(target, source) {
			return errors.New("restore destination must not equal, contain, or be inside the local backup folder")
		}
	}
	return nil
}

type Item struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type Point struct {
	Job         string `json:"job"`
	StoragePath string `json:"storage_path"`
	CreatedAt   string `json:"created_at"`
	BackupType  string `json:"backup_type"`
	Encryption  string `json:"encryption"`
	Locked      bool   `json:"locked"`
	LockedBy    string `json:"locked_by"`
	Items       []Item `json:"items"`
}
type Entry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}
type RestoreRequest struct {
	Connection  Connection
	Point       string
	Item        string
	Destination string
	Includes    []string
}

func (req RestoreRequest) Args() ([]string, error) {
	if req.Point == "" || req.Item == "" {
		return nil, errors.New("choose a recovery point and an item")
	}
	if err := validateDestination(req.Connection, req.Destination); err != nil {
		return nil, err
	}
	flags, err := req.Connection.Flags()
	if err != nil {
		return nil, err
	}
	dest, err := absPath(req.Destination)
	if err != nil {
		return nil, err
	}
	args := []string{"recover", "extract"}
	args = append(args, flags...)
	args = append(args, "--point", req.Point, "--item", req.Item, "--to", dest)
	for _, include := range req.Includes {
		if include == "" {
			return nil, errors.New("blank included file path")
		}
		args = append(args, "--include", include)
	}
	// Deliberately never offer --overwrite or --raw.
	return args, nil
}

type RunResult struct {
	Code        int
	Stdout      string
	Stderr      string
	Interrupted bool
}

func (r RunResult) Details() string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "Vault recovery executable exited with code %d.\n", r.Code)
	if r.Interrupted {
		fmt.Fprintln(b, "Stop requested. Partial recovered files may remain.")
	}
	if r.Stderr != "" {
		fmt.Fprintf(b, "\n--- actual Vault stderr ---\n%s", r.Stderr)
	}
	if r.Stdout != "" {
		fmt.Fprintf(b, "\n--- actual Vault stdout ---\n%s", r.Stdout)
	}
	return b.String()
}

type lockedWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	channel string
	notify  func(string, string)
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if w.notify != nil {
		w.notify(w.channel, string(p))
	}
	return n, err
}
func (w *lockedWriter) String() string { w.mu.Lock(); defer w.mu.Unlock(); return w.buf.String() }

// runOriginal: no shell, no subprocess of another program, no network by GUI.
// Scrub inherited passphrases so only an explicitly selected file is used.
func runOriginal(ctx context.Context, executable string, args []string, notify func(string, string)) (RunResult, error) {
	exe, err := absPath(executable)
	if err != nil {
		return RunResult{}, errors.New("select the official Vault executable")
	}
	st, err := os.Stat(exe)
	if err != nil {
		return RunResult{}, fmt.Errorf("Vault executable: %w", err)
	}
	if !st.Mode().IsRegular() {
		return RunResult{}, errors.New("Vault executable is not a regular file")
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	prepareVaultProcess(cmd) // suppress extra Windows console, preserve original executable
	cmd.Stdin = nil
	env := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.EqualFold(strings.SplitN(e, "=", 2)[0], "VAULT_PASSPHRASE") {
			env = append(env, e)
		}
	}
	cmd.Env = env
	out := &lockedWriter{channel: "stdout", notify: notify}
	errOut := &lockedWriter{channel: "stderr", notify: notify}
	cmd.Stdout = out
	cmd.Stderr = errOut
	err = cmd.Run()
	r := RunResult{Code: 0, Stdout: out.String(), Stderr: errOut.String(), Interrupted: ctx.Err() != nil}
	if err != nil {
		r.Code = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.Code = exitErr.ExitCode()
		}
		if r.Interrupted {
			return r, fmt.Errorf("%s", r.Details())
		}
		if _, ok := err.(*exec.ExitError); ok {
			return r, errors.New(r.Details())
		}
		return r, fmt.Errorf("failed to launch original Vault executable: %w\n%s", err, r.Details())
	}
	if r.Interrupted {
		return r, errors.New(r.Details())
	}
	return r, nil
}

func checkExecutable(ctx context.Context, exe string) (string, error) {
	result, err := runOriginal(ctx, exe, []string{"recover", "--help"}, nil)
	if err != nil {
		return "", err
	}
	help := result.Stdout + result.Stderr
	for _, cmd := range []string{"list", "contents", "extract"} {
		if !strings.Contains(help, cmd) {
			return "", fmt.Errorf("Vault CLI did not advertise recover %s", cmd)
		}
	}
	f, err := os.Open(exe)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err = io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func scanPoints(ctx context.Context, exe string, c Connection) ([]Point, RunResult, error) {
	flags, err := c.Flags()
	if err != nil {
		return nil, RunResult{}, err
	}
	args := append([]string{"recover", "list"}, flags...)
	args = append(args, "--json")
	res, err := runOriginal(ctx, exe, args, nil)
	if err != nil {
		return nil, res, err
	}
	var points []Point
	if err := json.Unmarshal([]byte(res.Stdout), &points); err != nil {
		return nil, res, fmt.Errorf("cannot read Vault list JSON: %w\n%s", err, res.Details())
	}
	if points == nil {
		return nil, res, errors.New("Vault did not return a JSON array of restore points")
	}
	for _, p := range points {
		if p.StoragePath == "" {
			return nil, res, errors.New("Vault list JSON has a record without storage_path")
		}
	}
	return points, res, nil
}
func inspectContents(ctx context.Context, exe string, c Connection, point, item string) ([]Entry, RunResult, error) {
	if point == "" || item == "" {
		return nil, RunResult{}, errors.New("choose a recovery point and item")
	}
	flags, err := c.Flags()
	if err != nil {
		return nil, RunResult{}, err
	}
	args := append([]string{"recover", "contents"}, flags...)
	args = append(args, "--point", point, "--item", item, "--json")
	res, err := runOriginal(ctx, exe, args, nil)
	if err != nil {
		return nil, res, err
	}
	var entries []Entry
	if err := json.Unmarshal([]byte(res.Stdout), &entries); err != nil {
		return nil, res, fmt.Errorf("cannot read Vault contents JSON: %w\n%s", err, res.Details())
	}
	if entries == nil {
		return nil, res, errors.New("Vault did not return a JSON array of files")
	}
	for _, p := range entries {
		if p.Path == "" {
			return nil, res, errors.New("Vault contents JSON has a record without path")
		}
	}
	return entries, res, nil
}

type Report struct {
	Point      string       `json:"point"`
	Items      []ItemReport `json:"items"`
	ReportPath string       `json:"-"` // verified local file, not a Vault JSON field
}
type ItemReport struct {
	Item    string `json:"item"`
	Type    string `json:"type"`
	Dir     string `json:"dir"`
	Files   int    `json:"files"`
	Bytes   int64  `json:"bytes"`
	Pruned  int    `json:"pruned,omitempty"`
	Renamed []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"renamed,omitempty"`
	Skipped []struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	} `json:"skipped,omitempty"`
	Error string `json:"error"`
}

var reportNameRE = regexp.MustCompile(`^vault-recover-report-[0-9]{8}-[0-9]{6}\.json$`)

func restore(ctx context.Context, exe string, req RestoreRequest, confirmed bool, notify func(string, string)) (Report, RunResult, error) {
	if !confirmed {
		return Report{}, RunResult{}, errors.New("restore blocked: explicit user confirmation is required")
	}
	args, err := req.Args()
	if err != nil {
		return Report{}, RunResult{}, err
	}
	started := time.Now()
	res, err := runOriginal(ctx, exe, args, notify)
	if err != nil {
		return Report{}, res, err
	}
	var reportPaths []string
	for _, line := range strings.Split(strings.ReplaceAll(res.Stdout, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "Report: ") {
			reportPaths = append(reportPaths, strings.TrimSpace(strings.TrimPrefix(line, "Report: ")))
		}
	}
	if len(reportPaths) != 1 {
		return Report{}, res, fmt.Errorf("Vault exited 0 but did not confirm exactly one recovery report. DO NOT assume recovery succeeded.\n%s", res.Details())
	}
	output, err := canonicalPath(req.Destination)
	if err != nil {
		return Report{}, res, err
	}
	reportFile, err := canonicalPath(reportPaths[0])
	if err != nil {
		return Report{}, res, fmt.Errorf("Vault report was not found: %w", err)
	}
	if !reportNameRE.MatchString(filepath.Base(reportFile)) || !samePath(filepath.Dir(reportFile), output) {
		return Report{}, res, fmt.Errorf("Vault reported a recovery report outside the selected output folder: %s", reportPaths[0])
	}
	stat, err := os.Stat(reportFile)
	if err != nil {
		return Report{}, res, fmt.Errorf("Vault report could not be read: %w", err)
	}
	if stat.ModTime().Before(started.Add(-3 * time.Second)) {
		return Report{}, res, errors.New("Vault recovery report predates this restore; cannot confirm success")
	}
	body, err := os.ReadFile(reportFile)
	if err != nil {
		return Report{}, res, fmt.Errorf("Vault report could not be read: %w", err)
	}
	var report Report
	if err := json.Unmarshal(body, &report); err != nil {
		return Report{}, res, fmt.Errorf("Vault report is invalid JSON: %w", err)
	}
	if report.Point != req.Point || len(report.Items) != 1 || report.Items[0].Item != req.Item || report.Items[0].Error != "" || report.Items[0].Files < 0 {
		return Report{}, res, fmt.Errorf("Vault report did not confirm recovery of the selected item. DO NOT assume success.\n%s", res.Details())
	}
	report.ReportPath = reportFile
	return report, res, nil
}
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
