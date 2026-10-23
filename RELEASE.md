# Release Process

Releases are cut automatically by GitHub Actions when a version tag is pushed.

## Creating a release

1. **Start from an up-to-date `main`:**

   ```bash
   git checkout main
   git pull origin main
   ```

2. **Add a section to `CHANGELOG.md`** for the version you are about to tag, in
   the `## [1.4.0] - 2026-02-01` form used throughout the file. The release
   workflow publishes the matching section as the release body, so it must describe
   the release you are tagging, not the previous one. The heading is matched
   against the tag with any leading `v` ignored — `## v1.4.0`, `## [1.4.0] - date`
   and `## 1.4.0` all work — and the section ends at the next `## ` heading. Add
   the real date when you tag, and replace `unreleased` with it. Omit a Downloads
   section — the workflow appends one generated from the tag.

   If no section matches the tag, the workflow warns and falls back to a body
   generated from `git log`.

3. **Pick the version.** The project follows semantic versioning:

   | Change | Version |
   | --- | --- |
   | A fix to behaviour that was wrong | patch (`v1.2.1`) |
   | A new flag, command or default | minor (`v1.3.0`) |
   | A breaking change to flags, output or file layout | major (`v2.0.0`) |

   Changing a default is user-visible even though no flag was added, so it is at
   least a minor bump unless the old default was simply broken.

4. **Tag and push:**

   ```bash
   ./scripts/release.sh v1.2.1
   ```

   Or by hand:

   ```bash
   git tag -a v1.2.1 -m "Release v1.2.1"
   git push origin v1.2.1
   ```

## What the workflow does

Pushing a `v*` tag runs `.github/workflows/release.yml`:

1. **Verify** — the full reusable CI pipeline (unit tests, integration tests
   against real PostgreSQL and SSH containers, and the installer checks). No
   binaries are built unless all of it passes.
2. **Build** — cross-compiles Linux and macOS binaries for amd64 and arm64, stamps
   the version, commit and build date into the binary, and smoke-tests that the
   binary reports the tag it was built from.
3. **Release** — publishes a GitHub release with the archives, `SHA256SUMS` and
   the installer attached.

## After the release

- The Homebrew tap in `andymarthin/homebrew-pgtransfer` is updated by its own daily
  cron job. To update it immediately, run that workflow by hand.
- Check the published release reports the new version:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | sh
  pgtransfer version
  ```

## If a release is broken

Delete the release and the tag, fix the problem on `main`, then tag again. Do not
reuse a version number that has already been downloaded: the archives are cached by
CDNs and by users, and a rebuilt archive with the same name will fail its checksum
for anyone who already fetched the old one.

```bash
gh release delete v1.2.1 --yes --cleanup-tag
```
