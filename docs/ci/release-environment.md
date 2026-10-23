# Protected release environment

Date: 2026-10-23. Status: documented, not applied. `PUT /repos/AlexGalhardo/kali-tools/environments/release` returned `403 Resource not accessible by integration`. `GET` of the environment list succeeded and reported no environments. The workflow file was not changed.

## Already in the workflow

`.github/workflows/ci.yml` grants `contents: read` to every job. The release job adds `contents: write`, `id-token: write` and `attestations: write`. It runs only when `github.ref == refs/heads/main` and the event is not `pull_request`. Pull requests cannot create releases.

## What a repository admin does

Create an environment named `release`. Required reviewer: `AlexGalhardo` (user id `19540357`). Leave "Prevent self-review" off. He is the only reviewer, and turning that option on would stop him approving a release he started. Limit deployment branches to `main`.

```bash
gh api --method PUT repos/AlexGalhardo/kali-tools/environments/release \
  --input - <<'EOF'
{
  "wait_timer": 0,
  "prevent_self_review": false,
  "reviewers": [{"type": "User", "id": 19540357}],
  "deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}
}
EOF
gh api --method POST repos/AlexGalhardo/kali-tools/environments/release/deployment-branch-policies \
  -f name=main -f type=branch
```

Those calls need a token that can administer the repository. The cloud-agent token cannot.

## Do not wire it into development drafts yet

The release job publishes a development draft on every main push. Adding this key would pause that job until Alex approves:

```yaml
environment: release
```

Add it only when a qualified release should wait. This note does not add it. R04's checkbox stays open.
