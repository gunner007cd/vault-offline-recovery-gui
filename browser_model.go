package main

// The file browser is a presentation-only index of paths supplied by the
// original `vault recover contents --json` command. It never opens archives,
// reads repository data, or extracts a file.

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type browserRow struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
}

type browserModel struct {
	entries        []browserRow
	folderChildren map[string][]string
	folders        map[string]bool
	byPath         map[string]browserRow
	direct         map[string][]browserRow
	selected       map[string]bool
	wholeItem      bool
}

func newBrowserModel(entries []Entry) *browserModel {
	b := &browserModel{
		folderChildren: make(map[string][]string),
		folders:        map[string]bool{"": true},
		byPath:         make(map[string]browserRow),
		direct:         make(map[string][]browserRow),
		selected:       make(map[string]bool),
	}
	for _, entry := range entries {
		// Preserve the exact strings from Vault for --include. Only accept
		// relative, canonical slash-delimited paths for browsing/selection.
		name := entry.Path
		if name == "" || name == "." || strings.HasPrefix(name, "/") || path.Clean(name) != name || strings.Contains(name, "\x00") {
			continue
		}
		parts := strings.Split(name, "/")
		invalid := false
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				invalid = true
				break
			}
		}
		if invalid {
			continue
		}
		if len(parts) == 0 {
			continue
		}
		for i := 1; i < len(parts); i++ {
			b.folders[strings.Join(parts[:i], "/")] = true
		}
		if entry.IsDir {
			b.folders[name] = true
		}
		b.byPath[name] = browserRow{Path: name, Name: path.Base(name), IsDir: entry.IsDir, Size: entry.Size}
	}
	// Include implicit directories in the display, too. They are valid
	// directory prefix targets for the existing Vault --include flag.
	for dir := range b.folders {
		if dir == "" {
			continue
		}
		if _, ok := b.byPath[dir]; !ok {
			b.byPath[dir] = browserRow{Path: dir, Name: path.Base(dir), IsDir: true}
		}
		parent := path.Dir(dir)
		if parent == "." {
			parent = ""
		}
		b.folderChildren[parent] = append(b.folderChildren[parent], dir)
	}
	for _, row := range b.byPath {
		b.entries = append(b.entries, row)
		parent := path.Dir(row.Path)
		if parent == "." {
			parent = ""
		}
		b.direct[parent] = append(b.direct[parent], row)
	}
	for parent, folders := range b.folderChildren {
		sort.Slice(folders, func(i, j int) bool { return strings.ToLower(folders[i]) < strings.ToLower(folders[j]) })
		b.folderChildren[parent] = folders
	}
	for parent, rows := range b.direct {
		sortRows(rows)
		b.direct[parent] = rows
	}
	sortRows(b.entries)
	return b
}

func sortRows(rows []browserRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].IsDir != rows[j].IsDir {
			return rows[i].IsDir
		}
		l, r := strings.ToLower(rows[i].Path), strings.ToLower(rows[j].Path)
		if l != r {
			return l < r
		}
		return rows[i].Path < rows[j].Path
	})
}

func (b *browserModel) children(folder string) []browserRow {
	return b.direct[folder]
}
func (b *browserModel) search(query string) []browserRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	result := make([]browserRow, 0)
	for _, row := range b.entries {
		if strings.Contains(strings.ToLower(row.Path), query) {
			result = append(result, row)
		}
	}
	return result
}
func (b *browserModel) add(p string) error {
	if p == "" {
		b.wholeItem = true
		b.selected = make(map[string]bool)
		return nil
	}
	if _, ok := b.byPath[p]; !ok {
		return fmt.Errorf("path %q was not returned by Vault", p)
	}
	if b.wholeItem {
		return fmt.Errorf("the entire item is checked; uncheck it before choosing individual paths")
	}
	for chosen := range b.selected {
		if chosen == p || strings.HasPrefix(p, chosen+"/") {
			return nil
		}
	}
	for chosen := range b.selected {
		if strings.HasPrefix(chosen, p+"/") {
			delete(b.selected, chosen)
		}
	}
	b.wholeItem = false
	b.selected[p] = true
	return nil
}
func (b *browserModel) remove(p string) {
	if p == "" {
		b.wholeItem = false
		return
	}
	delete(b.selected, p)
}

// checked reports checked state including a selected parent folder.
func (b *browserModel) checked(p string) bool {
	if b.wholeItem {
		return true
	}
	if b.selected[p] {
		return true
	}
	for ancestor := path.Dir(p); ancestor != "." && ancestor != ""; ancestor = path.Dir(ancestor) {
		if b.selected[ancestor] {
			return true
		}
	}
	return false
}

// toggle changes only explicit paths. Removing an inherited child while its
// parent is checked cannot be represented by the CLI's include-only flags;
// refuse to silently expand that selection into a different restore scope.
func (b *browserModel) toggle(p string) error {
	if p == "" {
		if b.wholeItem {
			b.remove("")
		} else {
			return b.add("")
		}
		return nil
	}
	if _, ok := b.byPath[p]; !ok {
		return fmt.Errorf("path %q was not returned by Vault", p)
	}
	if b.checked(p) {
		if !b.selected[p] {
			return fmt.Errorf("%s is included by a checked parent. Uncheck the parent folder first to select individual files", p)
		}
		b.remove(p)
		return nil
	}
	return b.add(p)
}
func (b *browserModel) clear() { b.selected = make(map[string]bool); b.wholeItem = false }

// afterConfirmedRestore retains the read-only catalog but discards the old
// recovery scope. This MUST only be called after the original Vault CLI has
// returned success AND its recovery report has been verified.
func (b *browserModel) afterConfirmedRestore() { b.clear() }
func (b *browserModel) ready() bool            { return b.wholeItem || len(b.selected) > 0 }
func (b *browserModel) includes() []string {
	out := make([]string, 0, len(b.selected))
	for p := range b.selected {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
