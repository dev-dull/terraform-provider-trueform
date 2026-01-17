# Setup Checklist for Publishing to Terraform Registry

Use this checklist to ensure everything is configured correctly before your first release.

## Repository Setup

- [ ] Repository is **public** (required for Terraform Registry)
- [ ] Repository is named `terraform-provider-trueform` (required naming convention)
- [ ] Branch protection is enabled on `main` branch

## GPG Key Setup

- [ ] GPG key generated (RSA 4096-bit recommended)
- [ ] Private key exported to `private-key.asc`
- [ ] Public key exported to `public-key.asc`
- [ ] Private key file deleted after adding to GitHub Secrets

## GitHub Secrets

Navigate to: **Repository Settings → Secrets and variables → Actions**

- [ ] `GPG_PRIVATE_KEY` - Full ASCII-armored private key
- [ ] `GPG_PASSPHRASE` - Passphrase for the GPG key

## Terraform Registry

1. [ ] Sign in at [registry.terraform.io](https://registry.terraform.io) with GitHub
2. [ ] Click **Publish → Provider**
3. [ ] Select the repository
4. [ ] Add GPG public key (contents of `public-key.asc`)
5. [ ] Complete registration

## Local Development

- [ ] Go 1.21+ installed
- [ ] GoReleaser installed (`brew install goreleaser`)
- [ ] golangci-lint installed (`brew install golangci-lint`)

## Verify Setup

```bash
# Build the provider
make build

# Run tests
make test

# Check GoReleaser config
make release-check

# Test release process (no publish)
make release-dry-run
```

## Create First Release

```bash
# Ensure all tests pass
make test

# Create and push tag
git tag -a v0.1.0 -m "Initial release"
git push origin v0.1.0

# Watch the release workflow in GitHub Actions
```

## Post-Release Verification

- [ ] GitHub Release created with all platform binaries
- [ ] SHA256SUMS file is signed
- [ ] Terraform Registry shows new version
- [ ] Test installation works:
  ```bash
  terraform init
  terraform providers
  ```

## Quick Commands

| Command | Description |
|---------|-------------|
| `make build` | Build provider binary |
| `make test` | Run tests |
| `make lint` | Run linter |
| `make release-dry-run` | Test release locally |
| `make dev-override` | Set up local dev environment |
