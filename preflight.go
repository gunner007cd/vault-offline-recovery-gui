package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// A read-only, advisory estimate from Vault's completed contents listing.
// This is NOT a prediction of filesystem overhead, temporary storage or
// compressed/deduplicated bytes and never replaces Vault's own checks.
type spaceEstimate struct {
	Files int
	Bytes int64
}

func estimateSelection(entries []Entry, wholeItem bool, includes []string) spaceEstimate {
	var out spaceEstimate
	unique := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir || e.Path == "" || unique[e.Path] {
			continue
		}
		selected := wholeItem
		if !selected {
			for _, inc := range includes {
				if e.Path == inc || strings.HasPrefix(e.Path, inc+"/") {
					selected = true
					break
				}
			}
		}
		if !selected {
			continue
		}
		unique[e.Path] = true
		out.Files++
		if e.Size > 0 {
			if e.Size > math.MaxInt64-out.Bytes {
				out.Bytes = math.MaxInt64
			} else {
				out.Bytes += e.Size
			}
		}
	}
	return out
}

func readableBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	i := 0
	for value >= 1024 && i < len(units)-1 {
		value /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", value, units[i])
}

// Return the nearest existing directory so a user can select a new, not-yet-
// created destination folder. The Windows volume query uses this directory.
func existingDestinationAncestor(dest string) (string, error) {
	p, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	for {
		info, err := os.Stat(p)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("%s is not a directory", p)
			}
			return p, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(p)
		if next == p {
			return "", fmt.Errorf("cannot find an existing parent directory for %s", dest)
		}
		p = next
	}
}

// Conflict check only: no directories are created or changed. Names Vault
// rewrites on Windows may differ, so this is an additional warning, not a
// guarantee. The Vault engine keeps its own no-overwrite protection.
func destinationItemConflict(dest, item string) string {
	if item == "" || item == "." || item == ".." || strings.ContainsAny(item, `/\\`) {
		return ""
	}
	folder := filepath.Join(dest, item)
	fi, err := os.Lstat(folder)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		return "Cannot inspect item destination: " + err.Error()
	}
	if !fi.IsDir() {
		return "Existing item path is not a directory: " + folder
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "Cannot inspect item destination: " + err.Error()
	}
	if len(entries) > 0 {
		return "The item's destination folder already contains files: " + folder + ". Vault does not overwrite by default."
	}
	return ""
}

func preflightSpaceMessage(estimate spaceEstimate, free int64, err error) string {
	if err != nil {
		return "Available destination space could not be determined (" + err.Error() + "). Verify capacity before proceeding."
	}
	if estimate.Files == 0 {
		return "No file size estimate is available; confirm that there is sufficient free space."
	}
	if free < estimate.Bytes {
		return fmt.Sprintf("WARNING: Only %s appears available, but the selected file sizes total %s. This restore may fail due to insufficient space.", readableBytes(free), readableBytes(estimate.Bytes))
	}
	if estimate.Bytes > 0 && free-estimate.Bytes < estimate.Bytes/5 {
		return fmt.Sprintf("Low free-space margin: %s available versus %s of selected file sizes. Extraction may require additional space.", readableBytes(free), readableBytes(estimate.Bytes))
	}
	return fmt.Sprintf("%s available on the destination drive (selected files: about %s).", readableBytes(free), readableBytes(estimate.Bytes))
}
