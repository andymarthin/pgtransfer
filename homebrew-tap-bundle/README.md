# Homebrew tap

The tap repository **`andymarthin/homebrew-pgtransfer`** is live and verified:

```bash
brew tap andymarthin/pgtransfer
brew install pgtransfer
pgtransfer version        # pgtransfer v1.1.0
```

`brew test` and `brew audit --strict` both pass.

## Remaining optional step: automatic tap updates

The tap already updates itself on a **daily schedule**, so new releases are
picked up without any configuration.

To make updates immediate, the main release workflow can notify the tap. It is
best-effort: when the secret below is absent the step is skipped and the daily
schedule still applies.

1. Create a token that can dispatch to the tap. A fine-grained personal access
   token scoped to **`andymarthin/homebrew-pgtransfer`** with
   **Contents: read and write** is enough.

2. Add it to the **main** repository as a secret named `HOMEBREW_TAP_TOKEN`:

   ```bash
   gh secret set HOMEBREW_TAP_TOKEN --repo andymarthin/pgtransfer
   ```

3. That is all — the next tagged release dispatches `pgtransfer-release`.

The dispatch path has been verified end to end: sending
`{"event_type":"pgtransfer-release","client_payload":{"version":"v1.1.0"}}`
to the tap triggered its workflow on `repository_dispatch`, which succeeded.

### Important: draft releases

The main release workflow creates releases as **drafts**. A draft's assets are
not publicly downloadable, so a tap update can only succeed once the draft is
published. Either:

- publish the draft in the GitHub UI, then re-run the release workflow
  (`gh workflow run Release --repo andymarthin/pgtransfer`), or
- rely on the tap's daily scheduled run, or
- change the release step to `draft: false` if you are comfortable with fully
  automatic releases.

## Reference: pushing the tap from scratch

`homebrew-pgtransfer.bundle` in this directory carries the initial commit. To
restore it (normally unnecessary — the repo already exists):

```bash
git clone homebrew-pgtransfer.bundle homebrew-pgtransfer
cd homebrew-pgtransfer
git remote remove origin
git remote add origin git@github.com:andymarthin/homebrew-pgtransfer.git
git push -u origin main
```

The repository name matters: Homebrew's short-form tap syntax (`brew tap
andymarthin/pgtransfer`) requires the repository to be named `homebrew-<name>`.

## Files

| Path | Purpose |
| --- | --- |
| `Formula/pgtransfer.rb` | Installs the prebuilt release archives for darwin/linux on arm64 and amd64 |
| `.github/workflows/update-formula.yml` | Rewrites URLs and checksums from the published `SHA256SUMS` |
| `README.md` | Tap install/upgrade/uninstall instructions |
