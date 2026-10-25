# ADR 014: Product identity

Status: accepted on 2026-10-25.

Context: an external audit (`specialist-feedback.md`, PA-01) noted that the name and the README sentence "inspired by the Kali Linux ecosystem" suggest an orchestrator of pentest tools. The code does not run Kali tools. ADR 006 removed Nmap, and PLAN section 1 excludes credential attacks, exploitation, Wi-Fi injection and evasion. The mismatch invites requests the project will not accept.

Decision:

- kali-tools is a tool for personal study of networking and information security. The author uses it on their own systems and lab to learn how networks, protocols and defenses behave.
- It is not a Kali tool orchestrator, a penetration-testing product or an engagement platform. It does not run Nmap, Metasploit, Hydra, sqlmap, Nikto or similar programs, and it does not ship exploitation, credential attacks, Wi-Fi injection or evasion.
- The name stays. The Kali Linux ecosystem is an inspiration for the learning topics, not a compatibility promise.
- Active operations stay bounded and need an explicit authorization confirmation (`--authorized`, the UI checkbox and the first-run notice). Use them only on systems you own or may test in writing.
- External-program orchestration (audit items FEAT-03 and FEAT-07) and mandatory engagement scope files (FEAT-01) are not planned. Reopen this ADR before adding any of them.

Consequences: README, PLAN and AGENTS describe the tool as a personal study toolkit. Roadmap work favors accuracy and clarity of the existing capabilities (port states, truncation, TLS and DNS inspection, honest capability states) over new offensive features. A rename stays possible but is not decided.

Revisit if the project gains users who need engagement-grade features, or if the author decides to rename the product.
