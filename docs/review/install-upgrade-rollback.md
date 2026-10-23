# Install, upgrade, rollback and uninstall

This is the procedure for a human on Windows x64, Ubuntu 24.04 x64 or macOS Apple Silicon. It was not run for this note. Runbook steps 13 and 15 remain the evidence path. A pasted result is still required before the R02 child can be checked.

There is no installer and no uninstaller. Archives are listed in [packaging](../packaging.md).

## Install

Unpack the archive for that OS. Run `kali-tools` or the desktop binary from the unpacked directory. The desktop app needs the WebView named in the packaging matrix. The CLI does not. Unpacking is not a qualified install.

## Upgrade

Unpack the newer archive beside the older one. Leave the older directory in place until the new binary has opened the existing database. The database is not inside the archive. A new database is schema version 1. There is no older schema to migrate. Opening the existing `history.db` is the upgrade.

## Rollback and recovery

Run the previous unpacked binary against the same database. There is no product backup command. Recovery is a copy of the closed database file, which the B06 fixture already reopens with its settings and finished job. Copy that file only while the application is closed.

## Uninstall

Delete the unpacked directory. `history.db` stays in the per-user config directory until the operator deletes it. Deleting the database removes settings and history.

## Offline

The application has no cloud account and does not send telemetry. It does not download a WebView. A machine without the WebView prerequisite cannot start the desktop app. That offline case was not tested here.

## What to record

OS build, archive name, whether the desktop app started, the path of `history.db` before and after unpacking a second archive, and whether the old binary still opened that file. Write "not run" if you only unpacked a CI artifact.
