# ADR 003: optional Nmap service inspection

Status: superseded by [ADR 006](006-native-service-inspection.md) on 2026-10-23. The text below is the historical design and is not the current implementation.

Native TCP discovery stays the default. An explicit service inspection uses a separately installed Nmap because its maintained service-probe/signature database supplies protocol identification that a TCP connection alone cannot provide. The application does not recreate that database or claim a port number identifies a service.

The supported command contract is Nmap 7.80 or newer within major 7. TShark 4.2 or newer within major 4 is recognized by inventory only; no TShark execution path is introduced. Unsupported versions remain unavailable. Neither executable is bundled.

Nmap receives fixed argument arrays for TCP connect scanning and light version detection. Scope is an explicit IPv4 /24 or narrower and up to 32 unique ports. No scripts, OS detection, user arguments, shell, UDP scan or scope expansion are exposed. A two-minute process deadline, 15-second host timeout, one retry and maximum scan-rate option bound the run. The scan-rate option does not establish a per-protocol application-probe rate guarantee.

Stdout is bounded to 4 MiB and stderr to 8 KiB; overflow cancels execution. Parsed hosts/ports must remain within the requested scope. XML must report a successful run and a supported provider version. Service labels retain the tool's detection method and are reported as provider observations.

Windows starts the executable suspended, attaches it to a kill-on-close Job Object, then resumes its primary thread. Cancellation terminates the job, including descendants. Unix uses a process group. Stderr is not copied into history or ordinary errors. History records provider version and counts; targets and probe content stay in memory.

Sources: [Nmap timing controls](https://nmap.org/book/man-performance.html), [Nmap service detection](https://nmap.org/book/man-version-detection.html), [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects), [AssignProcessToJobObject](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-assignprocesstojobobject).
