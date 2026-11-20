# Vault Offline Recovery GUI — v1.0

A native 64-bit Windows graphical interface for Tinkerologie's **official** standalone `vault recover` command. It works offline and is independent of Docker, Unraid, Python, and a running Vault server. It does **not** implement decryption, backup reading, or extraction, and does not modify the official Vault executable.

## Quick start

1. Put `Vault-Offline-Recovery-GUI-v1.0-Cancel-Label-Fix.exe` and the trusted official `vault-windows-amd64.exe` in the same folder. The GUI also supports browsing to an executable stored elsewhere.
2. Start the GUI; choose your backup folder and, when required, the original `vault.key` or an encrypted backup passphrase file. Never type a passphrase into a file/path label.
3. Choose **Scan backups** (read-only), then a restore point and item, then **Inspect item files** (read-only).
4. In the separate browser, tick the files or folders to restore. You can instead explicitly tick the item root. No checked paths means **no restore**.
5. Select an independent, preferably empty recovery destination, type `RESTORE`, and review the final confirmation carefully. The GUI checks destination free space and warns of known conflicts **before** starting Vault. The confirmation defaults to **No**.
6. The original executable performs recovery. A green/success mascot and recovery confirmation only appear after a successful process exit and validation of that run's original Vault recovery report. The new report-summary window offers **Open recovered folder** and **View Vault JSON report**.
7. To restore more files from the same inspected item, reopen the browser, check a fresh selection, and authorize another restore. File browsing remains instant; the original Vault executable may still rebuild its deduplication index for every new restore.

## v1.0 additions

- **3 minutes 30 seconds** before the impatient mascot expression appears. This is elapsed time, never an invented percent complete. The extraction expression is based on the original executable's stderr `extracting ...` milestone.
- **Pre-restore free-space comparison** based on indexed file sizes from the official `recover contents --json` output and Windows' actual available volume space. Sizes are **estimates** of file contents, not exact disk allocations. Compression, sparse files, metadata, temporary staging and filesystem overhead can change actual storage requirements. If a folder does not exist, the free-space query uses its nearest existing parent.
- **Explicit pre-restore review** of the selected restore point, backup timestamp, item, exact include paths, estimated size, destination, free space, and detected existing item folders. No overwrite option is ever passed to Vault. A conflict warning is an extra preflight check; the original Vault recovery engine remains authoritative.
- **Detailed confirmed recovery summary** sourced from Vault's *verified* JSON report: written files/bytes, renamed paths, skipped paths and pruned entries. A separate button opens the actual original report, which remains on disk. No GUI-computed "success" is substituted.
- **Optional path memory**: click `[ ] Remember paths on this PC` to opt in. Only the GUI executable path, local backup folder and last recovery destination are stored in `%APPDATA%\VaultOfflineRecovery\paths.json` (Windows user profile). Key paths, key data, passphrases, passphrase-file paths, remote settings/credentials and archive data are never stored. Toggle off to delete the settings file. Nothing is remembered unless you opt in.
- All nine expressive mascot states and the original GUI controls remain.

## Disaster recovery kit layout

```text
Vault-Disaster-Recovery/
  README.txt                           # your kit instructions
  Vault-Offline-Recovery-GUI-v1.0-Cancel-Label-Fix.exe  # this GUI
  vault-windows-amd64.exe             # official, unmodified Vault CLI
  vault.key                            # YOUR private original server key (if needed)
  other recovery notes / credentials  # keep private
```

**Security:** If your kit ZIP includes `vault.key`, anyone who obtains the ZIP and the backup repository may be able to recover protected backups. **Do not distribute or upload that ZIP.** Keep it securely offline and protect the kit with strong encryption (for example an encrypted archive or encrypted storage). Ordinary ZIP files are not inherently encrypted. Never use the recovery key in a publicly downloadable release.

The GUI-only distribution ZIP here intentionally contains **no** `vault.key` and **no** official Vault executable. Download the verified CLI from the original project; verify its SHA-256 against a trusted published release checksum before relying on it.

## Safety behavior

- Read-only repository access is performed by the original Vault recovery executable. The GUI never writes to backup storage.
- The original CLI remains usable independently: `vault-windows-amd64.exe recover --help`.
- No shell interpolation, homemade encryption, internal restore implementation, or `--overwrite` option.
- Nonzero CLI exit codes and stderr are reported as actual errors. A zero exit code without an appropriate, recent, on-destination, matching verified recovery report **does not count as success**.
- Stop command cancels the subprocess; partially extracted files may remain. Cancellations do not count as success.
- No guessed percent complete. The mascot does not indicate data integrity on its own.
- This Windows GUI is not a replacement for keeping backup copies and practicing real recovery.

## Verifying this release

A Go 1.23+ toolchain can rebuild from `source` with:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -H windowsgui" -o Vault-Offline-Recovery-GUI-v1.0-Cancel-Label-Fix.exe .
```

Tests: `go test ./...` and `go test -race ./...`. Run `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...` for Windows-specific static analysis. All tests run without the official recovery executable by using a local mock; a real Windows exercise with real backups should still be performed before treating this as a production release.

### v1.0 locked-backup warning update

Adds a ninth mascot state (confused expression with amber/orange ring) when a successful read-only scan returns **only locked restore points**, or when a locked restore point is selected among readable backups. The user-facing status clarifies that the original key or backup passphrase may be missing or incorrect. A locked point cannot be inspected or restored. Genuine CLI failures continue to show the red-ring error state; explicit cancellation still shows the blue-ring cancellation state.

This is a presentation-only update; the standalone Vault executable and the recovery adapter are unchanged. No keys or credentials are packaged.

### v1.0 scan-mascot regression fix

Restores the behavior verified in the original eight-state build: after a successful read-only scan finds readable restore points, the mascot **stays on the thinking expression** rather than returning to the opening salute. Selecting a readable point after viewing a locked one also returns to thinking. The amber locked expression still appears for an all-locked scan or a selected locked restore point; real failures, cancellation, and verified restore success retain their existing states. Changing the connection/key still resets the mascot to the opening salute and requires another scan.

The official Vault recovery executable and the GUI's recovery adapter are unchanged.

### v1.0 themed-dialog refinement

- Replaced the Windows white Yes/No pre-restore message box with a native dark charcoal/amber review window. It presents the **same preflight data** and remains **fail closed**. The owner window is disabled while reviewing, **Cancel** has initial keyboard focus, Escape or closing means no restore, and only clicking **Start restore** (or explicitly activating that button with the keyboard) authorizes the original Vault executable to run.
- The existing recovery-summary window now has a dedicated dialog background renderer. The old background renderer accidentally painted an additional mascot portrait in the summary window, causing the blue-ring photobomb at the edge. The corrected dialog renderer does not paint the mascot.
- All recovery engine subprocess operations, report verification, path safety checks, file selection and disk-space calculations are unchanged. This is a Windows native UI update only.

**Validation:** Automated Go tests, race detector, Windows vet and a Windows amd64 GUI cross-build passed. The new native dialog still requires a hands-on Windows visual/keyboard smoke test before replacing a proven production version.

### v1.0 Cancel button label fix

- Shortens the pre-restore review cancel button from `Cancel — no changes` to `Cancel` to prevent text truncation at Windows display scaling.
- No change to button width, event handlers, default focus, Escape behavior, modal logic, Vault operations, or verified-report checks.
