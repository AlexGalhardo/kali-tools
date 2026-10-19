# ADR 001: storage and native boundaries

Status: accepted for development, 2026-10-19.

## SQLite

Use modernc.org/sqlite 1.60.1. Its pure Go implementation avoids a C SQLite build dependency across native targets. mattn/go-sqlite3 is mature and MIT-licensed but requires CGO and a matching native compiler. modernc's SQLite translation adds binary size; the tradeoff must be measured before beta. The selected package declares BSD-3-Clause; SQLite itself is public domain. Preserve all transitive notices before redistribution.

Migrations run in a transaction. A single database connection and coordinated writer serialize writes. The initial schema has settings, jobs, diagnostic_results, tool_inventory and schema_migrations. The current application stores redacted job summaries only; raw results and packet bytes are not written. Interrupted jobs are cancelled on reopen. Preferences validate language and retention before writes.

## Native providers

Go net owns interface addresses and portable diagnostics. Windows IP Helper owns gateways, DNS and counters. WLAN APIs own current-network metadata and explicit saved-key retrieval. Windows ICMP APIs own IPv4 ping/traceroute. Linux procfs provides a partial route/counter provider; the other platform capabilities remain explicit unsupported responses.

The ordinary desktop process does not install or communicate with a privileged helper. Do not advertise the beta privilege-boundary task as implemented. Wi-Fi permission and capture authorization are checked by Windows.
