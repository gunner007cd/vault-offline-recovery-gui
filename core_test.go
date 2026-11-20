package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const fakeVault = `#!/bin/sh
if [ -n "$VAULT_PASSPHRASE" ]; then echo 'INHERITED SECRET' >&2; exit 19; fi
if [ "$1" != 'recover' ]; then echo 'unexpected command' >&2; exit 18; fi
mode="$2"; shift 2
point=""; item=""; dest=""
for arg in "$@"; do
 case "$arg" in '--overwrite'|'--raw') echo 'unsafe GUI option' >&2; exit 21;; esac
done
while [ "$#" -gt 0 ]; do
 case "$1" in
 --point) point="$2"; shift 2;;
 --item) item="$2"; shift 2;;
 --to) dest="$2"; shift 2;;
 *) shift;;
 esac
done
case "$mode" in
 --help) echo 'Recover: list contents extract';;
 list) printf '[{"job":"Docs","storage_path":"Docs/2026-10-01_020000","items":[{"name":"src","type":"folder"}]}]\n';;
 contents) printf '[{"path":"notes/a.txt","size":5},{"path":"notes","is_dir":true}]\n';;
 extract)
  if [ "$point" = 'FAIL/run' ]; then echo 'fatal: checksum mismatch in vault engine' >&2; exit 17; fi
  if [ "$point" = 'NOREPORT/run' ]; then echo 'warning: vault report missing' >&2; exit 0; fi
  mkdir -p "$dest" || exit 12
  if [ "$point" = 'BADREPORT/run' ]; then reported_item='wrong'; else reported_item="$item"; fi
  printf '{"point":"%s","items":[{"item":"%s","files":1,"bytes":5,"dir":"%s/src"}]}\n' "$point" "$reported_item" "$dest" > "$dest/vault-recover-report-20000101-000000.json"
  echo "Report: $dest/vault-recover-report-20000101-000000.json";;
 *) echo 'unknown subcommand' >&2;exit 18;;
esac
`

func fixture(t *testing.T) (string, Connection, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake test executable uses POSIX shell; Windows cross-compilation is tested separately")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "backups")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "fake-vault")
	if err := os.WriteFile(exe, []byte(fakeVault), 0700); err != nil {
		t.Fatal(err)
	}
	return exe, Connection{Type: "local", BackupFolder: source}, filepath.Join(dir, "output")
}
func TestReadOnlyListAndInspection(t *testing.T) {
	exe, conn, dest := fixture(t)
	hash, err := checkExecutable(context.Background(), exe)
	if err != nil || len(hash) != 64 {
		t.Fatalf("hash=%q err=%v", hash, err)
	}
	points, _, err := scanPoints(context.Background(), exe, conn)
	if err != nil || len(points) != 1 || points[0].StoragePath != "Docs/2026-10-01_020000" {
		t.Fatalf("points=%v err=%v", points, err)
	}
	entries, _, err := inspectContents(context.Background(), exe, conn, points[0].StoragePath, "src")
	if err != nil || len(entries) != 2 || entries[0].Path != "notes/a.txt" {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("read-only commands created destination: %v", err)
	}
}
func TestRequiresConfirmationAndEngineReport(t *testing.T) {
	exe, conn, dest := fixture(t)
	req := RestoreRequest{Connection: conn, Point: "Docs/2026-10-01_020000", Item: "src", Destination: dest}
	if _, _, err := restore(context.Background(), exe, req, false, nil); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("restore not blocked: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed restore created output: %v", err)
	}
	rep, _, err := restore(context.Background(), exe, req, true, nil)
	if err != nil || rep.Items[0].Files != 1 || rep.Items[0].Item != "src" || rep.ReportPath == "" {
		t.Fatalf("report=%+v err=%v", rep, err)
	}
}
func TestActualEngineFailureAndNoFalseSuccess(t *testing.T) {
	exe, conn, dest := fixture(t)
	for _, tc := range []struct{ point, needle string }{{"FAIL/run", "fatal: checksum mismatch in vault engine"}, {"NOREPORT/run", "did not confirm"}, {"BADREPORT/run", "did not confirm"}} {
		req := RestoreRequest{Connection: conn, Point: tc.point, Item: "src", Destination: dest}
		_, _, err := restore(context.Background(), exe, req, true, nil)
		if err == nil || !strings.Contains(err.Error(), tc.needle) {
			t.Fatalf("%s: got %v", tc.point, err)
		}
		if tc.point == "FAIL/run" && (!strings.Contains(err.Error(), "code 17") || !strings.Contains(err.Error(), "actual Vault stderr")) {
			t.Fatal("failed to surface original exit code/stderr")
		}
	}
}
func TestRejectBackupOutputOverlap(t *testing.T) {
	_, conn, dest := fixture(t)
	source := conn.BackupFolder
	for _, p := range []string{source, filepath.Join(source, "subfolder"), filepath.Dir(source)} {
		req := RestoreRequest{Connection: conn, Point: "Docs/run", Item: "src", Destination: p}
		if _, err := req.Args(); err == nil || !strings.Contains(err.Error(), "must not equal") {
			t.Fatalf("unsafe destination %q: %v", p, err)
		}
	}
	if _, err := (&RestoreRequest{Connection: conn, Point: "Docs/run", Item: "src", Destination: dest}).Args(); err != nil {
		t.Fatalf("safe destination rejected: %v", err)
	}
}
func TestNoOverwriteAndIncludePaths(t *testing.T) {
	_, conn, dest := fixture(t)
	req := RestoreRequest{Connection: conn, Point: "Docs/run", Item: "src", Destination: dest, Includes: []string{"folder,name/file.txt", "notes/a.txt"}}
	args, err := req.Args()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), "--overwrite") || strings.Contains(strings.Join(args, " "), "--raw") {
		t.Fatal("unsafe option used")
	}
	var got []string
	for i, arg := range args {
		if arg == "--include" && i+1 < len(args) {
			got = append(got, args[i+1])
		}
	}
	if len(got) != 2 || got[0] != "folder,name/file.txt" {
		t.Fatalf("include args: %q", got)
	}
}
func TestSecretsNotInherited(t *testing.T) {
	exe, conn, _ := fixture(t)
	t.Setenv("VAULT_PASSPHRASE", "secret-must-not-reach-child")
	if _, _, err := scanPoints(context.Background(), exe, conn); err != nil {
		t.Fatalf("inherited secret was passed to Vault: %v", err)
	}
}
func TestPathJunctionAvoidance(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX symlink fixture")
	}
	_, conn, _ := fixture(t)
	alias := filepath.Join(filepath.Dir(conn.BackupFolder), "alias")
	if err := os.Symlink(conn.BackupFolder, alias); err != nil {
		t.Skip(err)
	}
	for _, target := range []string{alias, filepath.Join(alias, "new/subdir")} {
		if err := validateDestination(conn, target); err == nil {
			t.Fatalf("accepted aliased source destination %q", target)
		}
	}
}

func TestReadOnlyInspectionCanBeCancelled(t *testing.T) {
	exe, conn, _ := fixture(t)
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	from := "contents) printf '[{\"path\":\"notes/a.txt\",\"size\":5},{\"path\":\"notes\",\"is_dir\":true}]\\n';;"
	to := "contents) while :; do :; done;;"
	changed := strings.Replace(string(body), from, to, 1)
	if changed == string(body) {
		t.Fatal("fixture no longer matches cancellation test")
	}
	if err := os.WriteFile(exe, []byte(changed), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, res, err := inspectContents(ctx, exe, conn, "Docs/2026-10-01_020000", "src")
	if err == nil || !res.Interrupted || !strings.Contains(err.Error(), "Stop requested") {
		t.Fatalf("cancelled inspection: result=%+v, error=%v", res, err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("read-only inspection took too long to stop")
	}
}

// Every GUI restore uses a fresh invocation of the original CLI. Reusing the
// read-only listing must never implicitly repeat the prior set of paths.
func TestSequentialRestoresFromCachedInspection(t *testing.T) {
	exe, conn, parent := fixture(t)
	b := newBrowserModel([]Entry{
		{Path: "config/one.txt", Size: 5},
		{Path: "config/two.txt", Size: 5},
		{Path: "other/three.txt", Size: 5},
	})
	if err := b.toggle("config/one.txt"); err != nil {
		t.Fatal(err)
	}
	first := RestoreRequest{
		Connection: conn, Point: "Docs/2026-10-01_020000", Item: "src",
		Destination: filepath.Join(parent, "recovery-1"), Includes: b.includes(),
	}
	if _, _, err := restore(context.Background(), exe, first, true, nil); err != nil {
		t.Fatalf("first recovery: %v", err)
	}
	b.afterConfirmedRestore()
	if b.ready() || len(b.includes()) != 0 {
		t.Fatalf("previous selection remained armed: %#v", b.includes())
	}
	if len(b.search(".txt")) != 3 {
		t.Fatal("cached inspection was lost after a confirmed restore")
	}
	if err := b.toggle("other/three.txt"); err != nil {
		t.Fatal(err)
	}
	second := RestoreRequest{
		Connection: conn, Point: "Docs/2026-10-01_020000", Item: "src",
		Destination: filepath.Join(parent, "recovery-2"), Includes: b.includes(),
	}
	args, err := second.Args()
	if err != nil {
		t.Fatal(err)
	}
	var includes []string
	for i, arg := range args {
		if arg == "--include" && i+1 < len(args) {
			includes = append(includes, args[i+1])
		}
	}
	if got := strings.Join(includes, ","); got != "other/three.txt" {
		t.Fatalf("second recovery reused previous include paths: %q", got)
	}
	if _, _, err := restore(context.Background(), exe, second, false, nil); err == nil {
		t.Fatal("second recovery skipped explicit confirmation")
	}
	if _, err := os.Stat(second.Destination); !os.IsNotExist(err) {
		t.Fatal("unconfirmed second recovery created output")
	}
	if _, _, err := restore(context.Background(), exe, second, true, nil); err != nil {
		t.Fatalf("second recovery: %v", err)
	}
	b.afterConfirmedRestore()
	if b.ready() {
		t.Fatal("second recovery left selection armed")
	}
}

func TestFailedRecoveryDoesNotClearUserChoice(t *testing.T) {
	exe, conn, dest := fixture(t)
	b := newBrowserModel([]Entry{{Path: "one.txt", Size: 5}})
	if err := b.toggle("one.txt"); err != nil {
		t.Fatal(err)
	}
	_, _, err := restore(context.Background(), exe, RestoreRequest{
		Connection: conn, Point: "FAIL/run", Item: "src", Destination: dest,
		Includes: b.includes(),
	}, true, nil)
	if err == nil {
		t.Fatal("expected original executable to fail")
	}
	// GUI calls afterConfirmedRestore only on a verified engine report, never
	// on failure: user can correct the destination and retry the same scope.
	if !b.ready() || len(b.includes()) != 1 || b.includes()[0] != "one.txt" {
		t.Fatal("failure incorrectly cleared the chosen files")
	}
}

// Ensure the UI's mascot can observe the *original executable's* stderr
// milestone while the same restore still performs its full report validation.
func TestRealCLIMilestoneSignalAndVerifiedReport(t *testing.T) {
	exe, conn, dest := fixture(t)
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), `mkdir -p "$dest" || exit 12`, `echo 'extracting src (folder, deduplicated) to output' >&2
  mkdir -p "$dest" || exit 12`, 1)
	if updated == string(body) {
		t.Fatal("test executable not patched")
	}
	if err := os.WriteFile(exe, []byte(updated), 0700); err != nil {
		t.Fatal(err)
	}
	var milestone extractorMilestone
	seen := 0
	req := RestoreRequest{Connection: conn, Point: "Docs/2026-10-01_020000", Item: "src", Destination: dest}
	report, _, err := restore(context.Background(), exe, req, true, func(channel, chunk string) {
		if channel == "stderr" && milestone.feed(chunk) {
			seen++
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 1 || report.Items[0].Files != 1 {
		t.Fatalf("milestone count=%d report=%+v", seen, report)
	}
}
