package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectionEstimateFoldersAndFilesNoDoubleCounting(t *testing.T) {
	e := []Entry{{Path: "alpha/a", Size: 10}, {Path: "alpha/b", Size: 20}, {Path: "beta/c", Size: 30}, {Path: "alpha/sub/d", Size: 40}, {Path: "alpha", IsDir: true}, {Path: "alpha/a", Size: 10}}
	got := estimateSelection(e, false, []string{"alpha", "alpha/a", "beta/c"})
	if got.Files != 4 || got.Bytes != 100 {
		t.Fatalf("unexpected estimate: %+v", got)
	}
	got = estimateSelection(e, true, nil)
	if got.Files != 4 || got.Bytes != 100 {
		t.Fatalf("whole item estimate: %+v", got)
	}
	got = estimateSelection(e, false, []string{"alpha/a"})
	if got.Files != 1 || got.Bytes != 10 {
		t.Fatalf("file estimate: %+v", got)
	}
}
func TestConflictNeverWrites(t *testing.T) {
	dir := t.TempDir()
	if x := destinationItemConflict(dir, "data"); x != "" {
		t.Fatal(x)
	}
	if err := os.Mkdir(filepath.Join(dir, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "existing"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if x := destinationItemConflict(dir, "data"); !strings.Contains(x, "already contains") {
		t.Fatal(x)
	}
}
func TestSummaryUsesVaultReportValues(t *testing.T) {
	r := Report{Point: "Flash/test", ReportPath: "C:/output/vault-recover-report-20261119-000000.json", Items: []ItemReport{{Item: "Flash Drive", Files: 9, Bytes: 2048}}}
	txt := recoverySummaryText(r)
	for _, x := range []string{"Flash/test", "Files written: 9", "2.0 KiB", "vault-recover-report"} {
		if !strings.Contains(txt, x) {
			t.Fatal("missing " + x)
		}
	}
}
func TestRememberPathsNoSecretPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "paths.json")
	if err := saveRememberedPaths(p, rememberedPaths{Executable: "C:/vault.exe", Backup: "D:/backups", Destination: "E:/restore"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"vault.key", "passphrase", "config_file", "password"} {
		if strings.Contains(strings.ToLower(string(body)), secret) {
			t.Fatalf("secret field %s persisted", secret)
		}
	}
	got, err := loadRememberedPaths(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Executable != "C:/vault.exe" || got.Backup != "D:/backups" {
		t.Fatalf("load: %+v", got)
	}
	if err := deleteRememberedPaths(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("still there %v", err)
	}
}
