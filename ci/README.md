# ci/

The scripts the CI workflows run. `rust/run.sh` is the Rust gate:
`.github/workflows/rust.yml` runs it, and so can you.

To change CI, edit `.github/workflows/` in a reviewed pull request.
Session credentials push workflow files (admin `DECISIONS.md` ADR-8, as
amended 2026-09-24), so staging a workflow here first for a maintainer
to promote is optional. Sessions still cannot push tags, so a maintainer
pushes any tag that a tag-triggered workflow needs.

Some of the workflows are maintained in admin as well, and an edit made
only in this repository does not last:

- A workflow with a template in admin `rollout/workflows/`, named
  `json__<file>`, changes in that template too, in a pull request to
  admin. Today that is `ci.yml`, `release.yml`, `crates-release.yml`,
  `notify-status.yml` and `scorecard.yml`. Admin `scripts/verify.sh`
  reports a deployed copy that differs from its template, and the next
  `rollout/apply-workflows.sh --apply` writes the template back over it.
- `clib.yml` and `clib-release.yml` are stamped from admin
  `tasks/clib-template/`, together with `go/clib/`. Change the template
  and restamp with admin `tasks/adopt-clib.sh`, which writes both
  workflows straight into `.github/workflows/`. The new stamp lands in
  this repository's own reviewed pull request. Admin `scripts/verify.sh`
  reports a stamped file that differs from its template.

The prose gate that was staged here, `workflows/docs.yml`, has been
promoted and runs from `.github/workflows/docs.yml`; `make prose` runs
the same check locally. See `docs/STYLE-GUIDE.md`.
