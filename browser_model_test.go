package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestBrowserNavigationAndSearch(t *testing.T) {
	b := newBrowserModel([]Entry{
		{Path: "config/nested/deep.txt", Size: 33},
		{Path: "config/abc.txt", Size: 5},
		{Path: "other/Another.TXT", Size: 7},
		{Path: "config", IsDir: true},
	})
	if len(b.folderChildren[""]) != 2 {
		t.Fatalf("roots=%v", b.folderChildren[""])
	}
	if len(b.children("config")) != 2 {
		t.Fatalf("config children=%v", b.children("config"))
	}
	res := b.search("TXT")
	if len(res) != 3 {
		t.Fatalf("search results=%v", res)
	}
	if res[0].Path == "" {
		t.Fatal("blank search result")
	}
	if _, ok := b.byPath["config/nested"]; !ok {
		t.Fatal("implicit nested directory unavailable")
	}
}
func TestBrowserSelectionSubtreeAndRemoval(t *testing.T) {
	b := newBrowserModel([]Entry{
		{Path: "a/one"}, {Path: "a/two"}, {Path: "b/three"},
	})
	if err := b.add("a/one"); err != nil {
		t.Fatal(err)
	}
	if err := b.add("a"); err != nil {
		t.Fatal(err)
	}
	if err := b.add("a/two"); err != nil {
		t.Fatal(err)
	}
	if err := b.add("b/three"); err != nil {
		t.Fatal(err)
	}
	if got, want := b.includes(), []string{"a", "b/three"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	b.remove("a")
	if got := b.includes(); !reflect.DeepEqual(got, []string{"b/three"}) {
		t.Fatal(got)
	}
	b.clear()
	if len(b.includes()) != 0 {
		t.Fatal("clear must return to whole-item selection")
	}
}
func TestBrowserRejectsUnlistedOrUnsafePaths(t *testing.T) {
	b := newBrowserModel([]Entry{{Path: "dir/good"}, {Path: "../bad"}, {Path: "/abs"}, {Path: "dir/../no"}})
	for _, s := range []string{"missing", "../bad", "/abs", "dir/../no"} {
		if err := b.add(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	if err := b.add("dir/good"); err != nil {
		t.Fatal(err)
	}
}
func TestBrowserThousandEntriesNoTruncation(t *testing.T) {
	entries := make([]Entry, 1041)
	for i := range entries {
		entries[i] = Entry{Path: fmt.Sprintf("root/file%04d.txt", i), Size: 32}
	}
	b := newBrowserModel(entries)
	if len(b.children("root")) != 1041 {
		t.Fatal("browser truncated listing")
	}
	if len(b.search("file10")) != 41 {
		t.Fatalf("search found %d", len(b.search("file10")))
	}
}

func TestBrowserCheckBoxesRequireExplicitSelection(t *testing.T) {
	b := newBrowserModel([]Entry{{Path: "f/a.txt"}, {Path: "f/b.txt"}, {Path: "other/c.txt"}})
	if b.ready() || b.checked("") || len(b.includes()) != 0 {
		t.Fatal("inspection must start with nothing checked")
	}
	if err := b.toggle("f/a.txt"); err != nil {
		t.Fatal(err)
	}
	if !b.ready() || !b.checked("f/a.txt") || b.checked("f/b.txt") {
		t.Fatal("individual file selection incorrect")
	}
	if err := b.toggle("f"); err != nil {
		t.Fatal(err)
	}
	if got := b.includes(); !reflect.DeepEqual(got, []string{"f"}) {
		t.Fatalf("folder must supersede child: %v", got)
	}
	if !b.checked("f/a.txt") || !b.checked("f/b.txt") || b.checked("other/c.txt") {
		t.Fatal("folder descendants should appear checked")
	}
	if err := b.toggle("f/a.txt"); err == nil {
		t.Fatal("cannot exclude inherited child with include-only flags")
	}
	if got := b.includes(); !reflect.DeepEqual(got, []string{"f"}) {
		t.Fatalf("failed toggle changed scope: %v", got)
	}
	if err := b.toggle("f"); err != nil {
		t.Fatal(err)
	}
	if b.ready() || b.checked("f/a.txt") {
		t.Fatal("unchecking parent must uncheck subtree")
	}
	if err := b.toggle(""); err != nil {
		t.Fatal(err)
	}
	if !b.wholeItem || !b.checked("other/c.txt") || !b.ready() || len(b.includes()) != 0 {
		t.Fatal("explicit whole item checkbox failed")
	}
	if err := b.toggle("other/c.txt"); err == nil {
		t.Fatal("cannot exclude individual item while whole selected")
	}
	if err := b.toggle(""); err != nil {
		t.Fatal(err)
	}
	if b.ready() || len(b.includes()) != 0 {
		t.Fatal("clearing root must not imply full recovery")
	}
}

func TestExplicitFolderInTree(t *testing.T) {
	b := newBrowserModel([]Entry{{Path: "top", IsDir: true}, {Path: "top/sub", IsDir: true}, {Path: "top/sub/a.txt"}})
	if len(b.folderChildren[""]) != 1 || len(b.folderChildren["top"]) != 1 {
		t.Fatalf("explicit directories missing from tree: %#v", b.folderChildren)
	}
}
