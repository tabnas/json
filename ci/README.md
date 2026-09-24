# ci/

Staging area for GitHub Actions workflow changes.

This directory exists because session credentials cannot write
`.github/workflows/*` — see admin `DECISIONS.md` ADR-8. To change CI:

1. Put the intended workflow file in `workflows/`.
2. A maintainer promotes it with the admin `rollout/apply-ci-folders.sh`
   script.

## Pending

Nothing. The prose gate that was staged here, `workflows/docs.yml`, has
been promoted and runs from `.github/workflows/docs.yml`; `make prose`
runs the same check locally. See `docs/STYLE-GUIDE.md`.
