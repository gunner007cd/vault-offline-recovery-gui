package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The Windows button used to be displayed and enabled, but its WM_COMMAND
// event was not handled. This source-contract test catches that exact regression
// even when the tests run on a non-Windows build host.
func TestOpenFileBrowserButtonIsWired(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "windows_gui.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		clause, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range clause.List {
			id, ok := expr.(*ast.Ident)
			if !ok || id.Name != "idOpenBrowser" {
				continue
			}
			for _, stmt := range clause.Body {
				ast.Inspect(stmt, func(nn ast.Node) bool {
					call, ok := nn.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "showBrowser" {
						return true
					}
					if receiver, ok := sel.X.(*ast.Ident); ok && receiver.Name == "a" {
						found = true
					}
					return true
				})
			}
		}
		return true
	})
	if !found {
		t.Fatal("Open file browser has no WM_COMMAND handler calling a.showBrowser()")
	}
}

func TestMultipleRestoresRetainFileCatalogAndRequireReselection(t *testing.T) {
	b := newBrowserModel([]Entry{{Path: "first/a.txt"}, {Path: "second/b.txt"}})
	if err := b.toggle("first"); err != nil {
		t.Fatal(err)
	}
	if !b.ready() {
		t.Fatal("first selection unavailable")
	}
	before := len(b.entries)
	b.afterConfirmedRestore()
	if b.ready() || len(b.includes()) > 0 || b.wholeItem {
		t.Fatal("previous restore selection was not cleared")
	}
	if len(b.entries) != before || len(b.children("second")) != 1 {
		t.Fatal("successful restore discarded the cached browser catalog")
	}
	if err := b.toggle("second/b.txt"); err != nil {
		t.Fatal(err)
	}
	if got := b.includes(); !reflect.DeepEqual(got, []string{"second/b.txt"}) {
		t.Fatalf("second restore selection = %v", got)
	}
}

// The locked state must be driven by the already parsed read-only scan result
// and user point selection, never by a fabricated CLI failure.
func TestLockedWarningsWiredIntoBothWindowsActions(t *testing.T) {
	body, err := os.ReadFile("windows_gui.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, snippet := range []string{
		"a.mascot.scanned(a.points)",
		"a.mascot.selectedLockedPoint(p.Locked)",
		"LOCKED / KEY NEEDED",
		"case \"scan\":",
	} {
		if !strings.Contains(source, snippet) {
			t.Errorf("missing locked warning UI hook %q", snippet)
		}
	}
}
