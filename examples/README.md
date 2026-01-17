# Trueform Provider Examples

This directory contains example Terraform configurations demonstrating how to use the Trueform provider to manage TrueNAS Scale resources.

## Quick Start

1. **Copy the example configuration:**
   ```bash
   cp main.tf my-config.tf
   ```

2. **Create a variables file:**
   ```bash
   cat > terraform.tfvars << 'EOF'
   truenas_host       = "192.168.1.100"
   truenas_api_key    = "your-api-key-here"
   truenas_verify_ssl = false
   EOF
   ```

3. **Initialize and apply:**
   ```bash
   terraform init
   terraform plan
   terraform apply
   ```

## Example Configuration

The `main.tf` file demonstrates:

| Resource Type | Description |
|---------------|-------------|
| **Data Sources** | Query existing pools, users, and datasets |
| **Dataset** | Create a ZFS dataset with compression and quota |
| **SMB Share** | Windows/CIFS file sharing |
| **NFS Share** | Unix/Linux file sharing |
| **Snapshot** | ZFS snapshot for backup/recovery |
| **User** | Local user account |
| **Cron Job** | Scheduled task execution |
| **Static Route** | Network routing configuration |
| **iSCSI** | Block storage (portal, initiator, target, extent) |
| **VM** | Virtual machine (commented out) |

## Provider Configuration

```hcl
provider "trueform" {
  host       = "192.168.1.100"    # TrueNAS IP or hostname
  api_key    = "1-xxxx..."        # API key from TrueNAS UI
  verify_ssl = false              # Set true for valid SSL certs
}
```

### Environment Variables

Alternatively, use environment variables:

```bash
export TRUENAS_HOST="192.168.1.100"
export TRUENAS_API_KEY="1-xxxx..."
export TRUENAS_VERIFY_SSL="false"
```

## Resource Examples

### Dataset

```hcl
resource "trueform_dataset" "media" {
  pool        = "tank"
  name        = "media"
  compression = "LZ4"           # Options: LZ4, GZIP, ZSTD, OFF
  quota       = 1099511627776   # 1TB in bytes
  comments    = "Media storage"
}
```

### SMB Share

```hcl
resource "trueform_share_smb" "media" {
  name      = "media"
  path      = "/mnt/tank/media"
  enabled   = true
  browsable = true
  guestok   = false
  ro        = false
}
```

### NFS Share

```hcl
resource "trueform_share_nfs" "media" {
  path     = "/mnt/tank/media"
  enabled  = true
  networks = ["192.168.1.0/24", "10.0.0.0/8"]
}
```

### iSCSI Target

```hcl
resource "trueform_iscsi_portal" "default" {
  comment = "Default portal"
  listen = [
    { ip = "0.0.0.0", port = 3260 }
  ]
}

resource "trueform_iscsi_target" "storage" {
  name  = "storage"
  alias = "Storage Target"
  groups = [
    { portal = trueform_iscsi_portal.default.id }
  ]
}
```

### Cron Job

```hcl
resource "trueform_cronjob" "backup" {
  user    = "root"
  command = "/usr/local/bin/backup.sh"
  enabled = true
  schedule = {
    minute = "0"
    hour   = "2"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
```

## Additional Examples

For more comprehensive examples including:
- Full CRUD lifecycle testing
- Resource modification examples
- Import examples
- Multi-resource configurations

See the **[test-resources/](../test-resources/)** directory:

| Directory | Description |
|-----------|-------------|
| `test-resources/create/` | Creates one of each resource type |
| `test-resources/modify/` | Demonstrates resource updates |
| `test-resources/INTEGRATION_TESTING.md` | Complete testing guide |

## Important Notes

### Value Formats

- **Compression**: Use uppercase values (`LZ4`, `GZIP`, `ZSTD`, `OFF`)
- **Deduplication**: Use uppercase (`ON`, `OFF`, `VERIFY`)
- **Sizes**: Specify in bytes (e.g., `1099511627776` for 1TB)
- **Networks**: Use CIDR notation (e.g., `192.168.1.0/24`)

### Nested Attributes

Use assignment syntax for nested attributes:

```hcl
# Correct - assignment syntax
schedule = {
  minute = "0"
  hour   = "2"
}

listen = [
  { ip = "0.0.0.0", port = 3260 }
]

# Incorrect - block syntax (will not work)
schedule {
  minute = "0"
  hour   = "2"
}
```

### Dependencies

Use `depends_on` when resources depend on others:

```hcl
resource "trueform_share_smb" "media" {
  path = "/mnt/tank/media"
  # ...
  depends_on = [trueform_dataset.media]
}
```

## Security

**Never commit sensitive files to version control:**

```gitignore
# Add to .gitignore
*.tfvars
*.tfstate
*.tfstate.*
```

## Troubleshooting

### Connection Issues

```bash
# Test connectivity
curl -k https://192.168.1.100/api/current
```

### Debug Logging

```bash
export TF_LOG=DEBUG
terraform apply
```

### API Documentation

Access TrueNAS API docs at: `https://<truenas-ip>/api/docs`
