# Packaging matrix

Native CI builds three unsigned archives. A clean machine has not confirmed that those archives start. Combinations absent from this table are not supported.

| Archive suffix | Host | Architecture | WebView prerequisite | What CI checks |
| --- | --- | --- | --- | --- |
| `windows-x64.zip` | `windows-2025` | x64 | Microsoft Edge WebView2 Runtime | compile, tests, Windows bridge smoke, archive upload |
| `linux-x64.tar.gz` | Ubuntu 24.04 | x64 | GTK 3 and WebKitGTK 4.1 | compile with `libpcap`, tests, archive upload |
| `macos-arm64.zip` | `macos-15` | arm64 | system WebKit, macOS 13 or later | compile, tests, archive upload |

`scripts/package.mjs` refuses a macOS package that is not arm64. Debian 13, Kali, other Linux distributions, Windows ARM, and Intel or universal macOS builds are not in this matrix. Debian and Kali remain best-effort for development and come with no guarantee ([ADR 010](adr/010-linux-baseline.md)).

## Archive contents

Each native archive contains the CLI (`kali-tools` or `kali-tools.exe`), the desktop binary or the single `.app`, `LICENSE.md`, `README.md`, `THIRD_PARTY_NOTICES.txt`, `dependencies.json` and `BUILD.json`.

`BUILD.json` records `signed: false`. The macOS value of `signature` is `ad-hoc`. Windows and Linux use `none`. That ad-hoc mark is the linker signature, not a Developer ID signature ([ADR 009](adr/009-signing-and-provenance.md)).

The main-branch release job adds `kali-tools-<version>-source.tar.gz`, loose copies of the license and notices, and `SHA256SUMS`. It also publishes a GitHub artifact attestation. Pull requests upload the three archives as CI artifacts and do not create a release.

## Install and uninstall

There is no installer and no uninstaller. Install means unpacking the archive and running the CLI or the desktop binary. Uninstall means deleting that unpacked directory.

Deleting the archive does not delete settings or history. The database is created at mode `0700` under the per-user config directory:

| OS | Path |
| --- | --- |
| Windows | `%AppData%\kali-tools\history.db` |
| Linux | `$XDG_CONFIG_HOME/kali-tools/history.db`, or `~/.config/kali-tools/history.db` |
| macOS | `~/Library/Application Support/kali-tools/history.db` |

If `kali-tools/history.db` does not exist and `hacker-tools/history.db` does, the legacy file is opened and is not moved.

## What this document does not close

CI runners already have the toolchain and, on Windows, a WebView. That is not a clean-machine smoke check. Launching the desktop app on hardware, including a missing WebView2 or WebKitGTK library, stays on the OS runbooks. Those steps do not check B07 until the pasted evidence is recorded.
