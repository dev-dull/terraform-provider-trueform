# Releasing

This document describes how to release new versions of the Trueform Terraform Provider.

## Overview

The release process is automated using GitHub Actions and GoReleaser. When a version tag is pushed to the repository, the release workflow automatically:

1. Builds the provider for multiple platforms (Linux, macOS, Windows, FreeBSD)
2. Creates release archives for each platform/architecture combination
3. Generates SHA256 checksums
4. Signs the checksums with GPG
5. Publishes the release to GitHub Releases

## Prerequisites

Before releasing, ensure the following secrets are configured in your GitHub repository:

| Secret | Description |
|--------|-------------|
| `GPG_PRIVATE_KEY` | ASCII-armored GPG private key for signing releases |
| `GPG_PASSPHRASE` | Passphrase for the GPG key |

The `GITHUB_TOKEN` is automatically provided by GitHub Actions.

## Version Numbering

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for new functionality in a backwards compatible manner
- **PATCH** version for backwards compatible bug fixes

### Version Tag Format

| Type | Tag Format | Example |
|------|------------|---------|
| Stable Release | `v{MAJOR}.{MINOR}.{PATCH}` | `v1.0.0`, `v1.2.3` |
| Beta Release | `v{MAJOR}.{MINOR}.{PATCH}-beta.{N}` | `v1.0.0-beta.1` |
| Release Candidate | `v{MAJOR}.{MINOR}.{PATCH}-rc.{N}` | `v1.0.0-rc.1` |
| Alpha Release | `v{MAJOR}.{MINOR}.{PATCH}-alpha.{N}` | `v1.0.0-alpha.1` |

## Releasing a Stable Version

1. **Ensure CI passes**: All tests must pass on the `main` branch.

2. **Update CHANGELOG** (if maintained): Document the changes in this release.

3. **Create and push the tag**:
   ```bash
   # Fetch latest changes
   git checkout main
   git pull origin main

   # Create an annotated tag
   git tag -a v1.0.0 -m "Release v1.0.0"

   # Push the tag to trigger the release
   git push origin v1.0.0
   ```

4. **Monitor the release**: Watch the [Actions tab](../../actions) for the release workflow to complete.

5. **Verify the release**: Check the [Releases page](../../releases) to ensure all artifacts were published correctly.

## Releasing a Beta Version

Beta releases are useful for testing new features before a stable release. They are automatically marked as "pre-release" on GitHub and won't be installed by default by Terraform users.

1. **Ensure CI passes**: All tests must pass on the branch you're releasing from.

2. **Create and push a beta tag**:
   ```bash
   # From your feature branch or main
   git checkout main
   git pull origin main

   # Create a beta tag
   git tag -a v1.0.0-beta.1 -m "Beta release v1.0.0-beta.1"

   # Push the tag
   git push origin v1.0.0-beta.1
   ```

3. **Subsequent beta releases**: Increment the beta number:
   ```bash
   git tag -a v1.0.0-beta.2 -m "Beta release v1.0.0-beta.2"
   git push origin v1.0.0-beta.2
   ```

### Using a Beta Version

Users can install a specific beta version by specifying the version constraint:

```hcl
terraform {
  required_providers {
    trueform = {
      source  = "trueform/trueform"
      version = "1.0.0-beta.1"
    }
  }
}
```

## Releasing a Release Candidate

Release candidates (RC) are used for final testing before a stable release:

```bash
git tag -a v1.0.0-rc.1 -m "Release candidate v1.0.0-rc.1"
git push origin v1.0.0-rc.1
```

## CI/CD Pipeline

### Continuous Integration (`ci.yml`)

Runs on every push to `main` and on pull requests:

| Job | Description |
|-----|-------------|
| `build` | Compiles the provider and runs unit tests |
| `lint` | Runs golangci-lint for code quality |
| `terraform-validate` | Validates Terraform configurations in test directories |
| `goreleaser-check` | Validates the GoReleaser configuration |

### Release Workflow (`release.yml`)

Triggered when a tag matching `v*` is pushed:

1. **Checkout**: Fetches the repository with full history
2. **Setup Go**: Installs the Go version specified in `go.mod`
3. **Import GPG Key**: Loads the signing key from secrets
4. **GoReleaser**: Builds, signs, and publishes the release

### Build Matrix

The provider is built for the following platforms:

| OS | Architectures |
|----|---------------|
| Linux | amd64, arm64, arm (v6, v7), 386 |
| macOS | amd64, arm64 |
| Windows | amd64, 386 |
| FreeBSD | amd64, arm, 386 |

## Troubleshooting

### Release workflow failed

1. Check the [Actions tab](../../actions) for error details
2. Common issues:
   - Missing or invalid GPG secrets
   - GoReleaser configuration errors
   - Build failures

### GPG signing failed

Ensure the GPG key is properly configured:

```bash
# Export your GPG key (run locally)
gpg --armor --export-secret-keys YOUR_KEY_ID
```

Add the output as the `GPG_PRIVATE_KEY` secret in GitHub.

### Tag already exists

If you need to re-release:

```bash
# Delete the local tag
git tag -d v1.0.0

# Delete the remote tag
git push origin :refs/tags/v1.0.0

# Delete the GitHub release manually via the web UI

# Re-create and push the tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

## Post-Release

After a successful release:

1. **Announce the release**: Update any relevant documentation or communication channels
2. **Monitor issues**: Watch for bug reports related to the new version
3. **Terraform Registry**: The release will be automatically picked up by the Terraform Registry (if configured)

## Local Testing

To test the release process locally without publishing:

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Run a snapshot build (no publishing)
goreleaser release --snapshot --clean

# Check the dist/ directory for build artifacts
ls -la dist/
```
