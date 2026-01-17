# Releasing the Trueform Provider

This document describes how to set up and perform releases of the Terraform Provider for TrueNAS.

## Prerequisites

Before you can publish releases, you need to configure several things:

1. **GPG Key** - For signing release artifacts
2. **GitHub Secrets** - For the release workflow
3. **Terraform Registry** - For publishing the provider

---

## Step 1: Create a GPG Key

The Terraform Registry requires signed releases. Create a GPG key specifically for signing.

### Generate the Key

```bash
# Generate a new GPG key
gpg --full-generate-key

# Select:
# - (1) RSA and RSA
# - Key size: 4096
# - Expiration: 0 (does not expire) or your preference
# - Real name: Your Name or "Trueform Release Signing"
# - Email: your-email@example.com
# - Comment: (optional) "Terraform Provider Signing Key"
```

### Export the Private Key

```bash
# List your keys to find the key ID
gpg --list-secret-keys --keyid-format=long

# Output will look like:
# sec   rsa4096/ABCD1234EFGH5678 2024-01-01 [SC]
#       FINGERPRINT1234567890ABCDEF1234567890ABCDEF
# uid                 [ultimate] Your Name <your-email@example.com>
# ssb   rsa4096/WXYZ9876STUV5432 2024-01-01 [E]

# Export the private key (replace KEY_ID with your key ID, e.g., ABCD1234EFGH5678)
gpg --armor --export-secret-keys KEY_ID > private-key.asc

# View the contents - you'll need this for GitHub Secrets
cat private-key.asc
```

### Export the Public Key (for Terraform Registry)

```bash
# Export the public key
gpg --armor --export KEY_ID > public-key.asc

# View the contents - you'll need this for Terraform Registry
cat public-key.asc
```

> **Important:** Keep `private-key.asc` secure and delete it after adding to GitHub Secrets.

---

## Step 2: Configure GitHub Secrets

Add the following secrets to your GitHub repository:

### Navigate to Repository Settings

1. Go to your GitHub repository
2. Click **Settings** > **Secrets and variables** > **Actions**
3. Click **New repository secret**

### Required Secrets

| Secret Name | Description | How to Get |
|-------------|-------------|------------|
| `GPG_PRIVATE_KEY` | ASCII-armored private GPG key | Contents of `private-key.asc` |
| `GPG_PASSPHRASE` | Passphrase for the GPG key | The passphrase you set when creating the key |

### Adding the GPG Private Key

1. Click **New repository secret**
2. Name: `GPG_PRIVATE_KEY`
3. Value: Paste the entire contents of `private-key.asc`, including:
   ```
   -----BEGIN PGP PRIVATE KEY BLOCK-----
   ... (key content) ...
   -----END PGP PRIVATE KEY BLOCK-----
   ```
4. Click **Add secret**

### Adding the GPG Passphrase

1. Click **New repository secret**
2. Name: `GPG_PASSPHRASE`
3. Value: Your GPG key passphrase
4. Click **Add secret**

> **Note:** `GITHUB_TOKEN` is automatically provided by GitHub Actions - you don't need to create it.

---

## Step 3: Set Up Terraform Registry

### Rename Repository (if needed)

The Terraform Registry requires repositories to follow the naming convention:

```
terraform-provider-{NAME}
```

If your repository isn't named correctly, rename it:

1. Go to **Settings** > **General**
2. Under "Repository name", change to `terraform-provider-trueform`

### Sign Up for Terraform Registry

1. Go to [registry.terraform.io](https://registry.terraform.io/)
2. Click **Sign In** and authenticate with GitHub
3. Click **Publish** > **Provider**

### Add Your Provider

1. Select your GitHub account/organization
2. Select the `terraform-provider-trueform` repository
3. Add your GPG public key:
   - Paste the contents of `public-key.asc`
4. Click **Publish Provider**

### Provider Address

Once published, users can reference your provider as:

```hcl
terraform {
  required_providers {
    trueform = {
      source  = "YOUR_NAMESPACE/trueform"
      version = "~> 1.0"
    }
  }
}
```

Where `YOUR_NAMESPACE` is your GitHub username or organization name.

---

## Step 4: Creating a Release

### Semantic Versioning

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (v1.0.0 → v2.0.0): Breaking changes
- **MINOR** (v1.0.0 → v1.1.0): New features, backward compatible
- **PATCH** (v1.0.0 → v1.0.1): Bug fixes, backward compatible

### Create and Push a Tag

```bash
# Ensure you're on main and up to date
git checkout main
git pull origin main

# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to GitHub
git push origin v1.0.0
```

### Automated Release Process

When you push a tag:

1. GitHub Actions triggers the `release.yml` workflow
2. GoReleaser builds binaries for all platforms
3. Artifacts are signed with your GPG key
4. A GitHub Release is created with:
   - Release notes (auto-generated from commits)
   - Signed checksums file
   - Platform-specific zip archives
5. Terraform Registry automatically detects the new release

### Monitor the Release

1. Go to **Actions** tab in GitHub
2. Watch the "Release" workflow
3. Once complete, check **Releases** to see the published release

---

## Troubleshooting

### GPG Signing Fails

**Error:** `gpg: signing failed: No secret key`

**Solution:** Ensure `GPG_PRIVATE_KEY` secret contains the full private key block.

### GoReleaser Fails

**Error:** `could not import private key: openpgp: invalid data: private key checksum failure`

**Solution:** Regenerate the GPG key and re-export it. The passphrase might be incorrect.

### Terraform Registry Doesn't Detect Release

**Possible causes:**
- Repository name doesn't match `terraform-provider-{NAME}` pattern
- GPG signature verification failed
- Release artifacts missing required files

**Solution:** Check that all required files are in the release:
- `terraform-provider-trueform_X.Y.Z_SHA256SUMS`
- `terraform-provider-trueform_X.Y.Z_SHA256SUMS.sig`
- Platform-specific zip files

### Pre-release Versions

For pre-release versions (alpha, beta, RC):

```bash
git tag -a v1.0.0-alpha.1 -m "Pre-release v1.0.0-alpha.1"
git push origin v1.0.0-alpha.1
```

GoReleaser will automatically mark these as pre-releases.

---

## Release Checklist

Before creating a release:

- [ ] All tests pass (`go test ./...`)
- [ ] Code is linted (`golangci-lint run`)
- [ ] CHANGELOG is updated (if manual)
- [ ] Documentation is current
- [ ] Version number follows semver
- [ ] Previous release issues are resolved

After creating a release:

- [ ] GitHub Release was created
- [ ] All platform binaries are present
- [ ] Checksums file is signed
- [ ] Terraform Registry shows new version
- [ ] Test installation: `terraform init`

---

## Local Testing

Test the release process locally without publishing:

```bash
# Install GoReleaser
brew install goreleaser

# Test the build (no publish)
goreleaser release --snapshot --clean

# Check the dist/ directory for artifacts
ls -la dist/
```

---

## Security Notes

1. **Never commit** private keys or secrets to the repository
2. **Rotate GPG keys** periodically (yearly recommended)
3. **Use a dedicated GPG key** for signing releases
4. **Enable branch protection** on `main` to prevent unauthorized releases
5. **Review Actions logs** for any exposed secrets (GitHub automatically masks them)
