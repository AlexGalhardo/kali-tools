# Contributor instructions

Use English for source, documentation and commits. Keep business logic in Go services shared by CLI and Wails. Never concatenate shell commands. Bound network operations, parser input, event queues and subprocess output. Never persist Wi-Fi secrets or packet payloads in diagnostic history. GitHub Actions activation was explicitly authorized on 2026-10-19; maintain pinned workflows under .github/workflows with minimum permissions. Mark PLAN.md tasks complete only with actual verification evidence. Run `go test ./...`, `go vet ./...` and the frontend check/build after relevant changes. Do not claim native platform acceptance from cross compilation. Use the MIT project license. No release tag before qualification and owner decisions on signing. Development artifacts must disclose unsigned status and open acceptance gates.

## Project skills

Apply the repository copies of Impeccable for interface work, karpathy-guidelines for focused implementation and Ponytail for the smallest complete solution. Read their SKILL.md files under .agents/skills before using them. Source revisions are recorded in .agents/skills/sources.json. User scope and instructions take precedence; do not invent product requirements or mark hardware acceptance complete from compilation.
