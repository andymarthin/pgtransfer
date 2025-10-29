# Release Process

This document describes how to create releases for PGTransfer.

## Automated Release Process

PGTransfer uses GitHub Actions to automatically build and create releases when version tags are pushed to the repository.

### Creating a Release

1. **Ensure you're on the main branch and it's up to date:**
   ```bash
   git checkout main
   git pull origin main
   ```

2. **Run tests to ensure everything is working:**
   ```bash
   go test -v ./...
   ```

3. **Use the release script (recommended):**
   ```bash
   ./scripts/release.sh v1.0.0
   ```
   
   Or manually create and push a tag:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

4. **Monitor the GitHub Actions workflow:**
   - Go to the Actions tab in your GitHub repository
   - Watch the "Release" workflow complete
   - The workflow will automatically create a GitHub release with binaries

### Version Naming Convention

Use semantic versioning (SemVer) with a `v` prefix:
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)
- `v1.0.0-beta` - Pre-release

### What the Automated Process Does

1. **Triggers on tag push:** The workflow runs when you push a tag starting with `v`

2. **Cross-platform builds:** Creates binaries for:
   - Linux (x64 and ARM64)
   - macOS (Intel and Apple Silicon)

3. **Version information:** Embeds version, commit hash, and build date into binaries

4. **Creates archives:** Packages binaries into `.tar.gz` files for easy distribution

5. **Generates changelog:** Automatically creates release notes from git commits

6. **Creates GitHub release:** Publishes the release with all artifacts attached

### Build Artifacts

Each release includes the following files:
- `pgtransfer-v1.0.0-linux-amd64.tar.gz` - Linux x64
- `pgtransfer-v1.0.0-linux-arm64.tar.gz` - Linux ARM64
- `pgtransfer-v1.0.0-darwin-amd64.tar.gz` - macOS Intel
- `pgtransfer-v1.0.0-darwin-arm64.tar.gz` - macOS Apple Silicon

### Version Information

Users can check the version of their binary:
```bash
pgtransfer --version
# Output:
# pgtransfer v1.0.0
# Commit: abc1234
# Built: 2024-01-15T10:30:00Z
```

### Docker Images (Optional)

The workflow also includes a Docker build step that can be enabled by:
1. Uncommenting the Docker Hub login section
2. Adding `DOCKER_USERNAME` and `DOCKER_PASSWORD` secrets to your repository
3. Setting `push: true` in the Docker build step

### Troubleshooting

**Release workflow fails:**
- Check that all tests pass locally
- Ensure the tag follows the correct format (`v*`)
- Verify that the repository has the necessary permissions

**Missing artifacts:**
- Check the workflow logs in the Actions tab
- Ensure the build step completed successfully for all platforms

**Version not embedded:**
- Verify that the ldflags in the workflow are correct
- Check that the version variables are defined in main.go

### Manual Release (Not Recommended)

If you need to create a release manually:

1. Build for all platforms:
   ```bash
   # Linux x64
   GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=v1.0.0" -o pgtransfer-linux-amd64
   
   # Linux ARM64
   GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.version=v1.0.0" -o pgtransfer-linux-arm64
   
   # macOS Intel
   GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.version=v1.0.0" -o pgtransfer-darwin-amd64
   
   # macOS Apple Silicon
   GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.version=v1.0.0" -o pgtransfer-darwin-arm64
   ```

2. Create archives and upload to GitHub releases manually

However, using the automated process is strongly recommended for consistency and reliability.