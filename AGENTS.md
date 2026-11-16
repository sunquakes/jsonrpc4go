# AGENTS

## Releasing

When the user defines a release version (`x.y.z`), execute these steps in this
exact order. Do not stop halfway and do not reorder them:

1. **Add the changelog entry.** Update `CHANGELOG.md`: rename the
   `## [Unreleased]` heading to `## [vx.y.z] - YYYY-MM-DD`, keep the accumulated
   sections (do not use placeholder text), and add a fresh empty
   `## [Unreleased]` heading on top. If the file has no `## [Unreleased]`
   heading yet, add one above the newest release. This repo is a Go module, so
   the heading and the tag both use the `vx.y.z` form.
2. **Commit and push to `develop`.** Commit the release preparation as
   `chore: release version vx.y.z` and push it (`git push origin develop`).
3. **Create and merge the pull request to `main`.** The `gh` CLI is installed and
   authenticated as `sunquakes`. Run
   `gh pr create --base main --head develop --title "chore: release version vx.y.z" --body "..."`
   and merge the new PR with `gh pr merge <number> --merge`.
4. **Create the tag on `main`.** After the PR is merged, switch to `main`, pull,
   and create a lightweight tag `vx.y.z` on the merge commit
   (`git tag vx.y.z`). Stash unrelated local edits first if `git pull` refuses to
   run.
5. **Push the tag to the remote.** `git push origin vx.y.z` - this is the last
   step because pushing the tag triggers the `Create Release` workflow (GitHub
   Release, docs deploy). Verify that the workflow run completes.

Never create or push a release tag before the code is merged to `main`, never
tag from `develop`, and never publish a release manually.
