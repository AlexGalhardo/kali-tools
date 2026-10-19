# Security policy

This development alpha has no stable security support commitment yet. Report suspected vulnerabilities privately to the repository owner through GitHub private vulnerability reporting where enabled. Do not publish real passwords, captured packets, tokens or personal diagnostic data in issues.

Input boundaries include Wails methods, diagnostic targets, capture bytes, SQLite migrations and external program output. Diagnostics have deadlines and scope limits. The UI never receives arbitrary SQL or command execution methods. Wi-Fi reveal is explicit, ephemeral and excluded from history. Capture requires an explicit start and is never started on application launch.

The application does not elevate itself or install a privileged helper. Windows raw capture reports when administrator authorization is required. Do not treat this alpha as a hardened privileged application; the privilege boundary and native acceptance remain release gates in PLAN.md.
