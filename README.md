# Vault Offline Recovery GUI

A standalone, 64-bit Windows GUI for [Tinkerologie Vault](https://github.com/tinkerologie/vault)'s original `vault recover` command. It lets you inspect backup points, browse files, and perform selective restores without a running Vault server, Docker, or Unraid.

The GUI does **not** implement decryption or extraction. It starts the unmodified, official Vault recovery executable and reports its results. This is an independent companion utility, not a replacement for Vault.

## Download

For a ready-to-run Windows build, check [GitHub Releases](https://github.com/gunner007cd/vault-offline-recovery-gui/releases) for:

- `vault-offline-recovery-gui.exe` — the Windows application
- `vault-offline-recovery-gui.exe.sha256` — its SHA-256 checksum

The executable is unsigned, so Windows SmartScreen may display a reputation warning. Verify the download and only run software you trust.

**The original Vault recovery executable is a separate requirement.** Download `vault-windows-amd64.exe` from the [original Vault releases](https://github.com/tinkerologie/vault/releases), and compare its SHA-256 with the checksum published by that project. The GUI's **Check executable + SHA-256** button reports the *Vault CLI's* hash for that comparison; it does not check the GUI's own hash.

## Use the GUI

1. Put `vault-offline-recovery-gui.exe` and the verified `vault-windows-amd64.exe` together in a folder, or select the Vault executable manually in the first field.
2. Choose a storage type and select a local backup folder (or supply a supported remote-storage settings JSON).
3. If the backups are protected, supply the original server's `vault.key` and/or a **passphrase file**. The passphrase file must contain **only the passphrase itself** on one line. If you have a Vault Emergency Kit, copy its passphrase into a separate file; do not select the whole Emergency Kit text.
4. Select **Scan backups (read only)**, choose a readable restore point and item, and select **Inspect item files (read only)**.
5. Open the file browser and tick the file(s) or folder(s) to restore. An empty selection does not initiate recovery.
6. Select an independent recovery destination, type `RESTORE`, and inspect the pre-restore review. The restore starts only after explicit confirmation.
7. After a confirmed successful extraction, open the recovered folder or view the JSON recovery report written by the original Vault executable.

Deduplicated repositories can be unlocked using the original `vault.key` or a configured backup passphrase recovery mechanism. A passphrase only works where it was configured for that repository. Encrypted classic backups use the backup passphrase. Recovery is not possible without the appropriate key material.

### Safety behavior

- Backup storage is opened read-only by the original Vault recovery command; the GUI does not alter the repository.
- Restoring requires a checked selection, `RESTORE` text confirmation, and a separate review of selected data and available destination space.
- The GUI does not request Vault's `--overwrite` option. Use a new, preferably empty destination.
- A success message requires a successful Vault process exit **and** a matching, verified recovery report. Failures and cancellations are not reported as successful restores.
- Stopping a restore may leave partially extracted files in the destination.
- The optional **Remember paths on this PC** setting only saves nonsecret paths locally. It does not save the backup key, passphrase, key-file path, passphrase-file path, or remote credentials.

Keep your original `vault.key`, Emergency Kit, and other recovery secrets securely **outside this repository**. They are not packaged with the GUI or with GitHub releases.

## Build from source on Windows

The Go source, tests, embedded image assets, and Windows icon resource are all in the **repository root**. No `source/` directory, Python environment, external GUI library, or custom asset-generation step is required.

Requirements: Windows x64, [Go 1.23 or newer](https://go.dev/dl/), and Git.

In **PowerShell**:

```powershell
git clone https://github.com/gunner007cd/vault-offline-recovery-gui.git
cd vault-offline-recovery-gui
go test ./...
go build -buildvcs=false -trimpath -ldflags="-s -w -H windowsgui" -o vault-offline-recovery-gui.exe .
```

The output is `vault-offline-recovery-gui.exe` in the repository folder. The nine UI images and icon resource are embedded in the built executable. The original `vault-windows-amd64.exe` is **not** embedded and must be obtained separately.

To compute a checksum for your own build:

```powershell
(Get-FileHash .\vault-offline-recovery-gui.exe -Algorithm SHA256).Hash
```

A locally rebuilt executable may have a different hash if the Go toolchain or build environment differs from the published release.

## Releases and source archive

This repository tracks the source code and build configuration. Compiled `.exe` files are intentionally excluded from Git commits by `.gitignore`; **GitHub Release assets are downloadable separately and are not affected by `.gitignore`**. Release builds include a standalone EXE and a companion checksum. GitHub also automatically offers source archives for each release tag.

The GitHub Actions workflow at `.github/workflows/windows-release.yml` builds and publishes releases. It can be run manually from the **Actions** tab with a new version tag. Release binaries are separate from the original Vault project and are not code-signed.

## Limitations

This is a Windows GUI for the existing `vault recover` interface. It depends on the CLI version supporting the expected `list`, `contents`, and `extract` commands and JSON output. Test compatibility after updating the original Vault executable, and retain a known-good recovery kit.
