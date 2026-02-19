# Integration Testing Guide

This directory contains Terraform configurations to test all resource types in the Trueform provider through the complete lifecycle: **create → import → modify → destroy**.

## Directory Structure

```
test-resources/
├── README.md                    # This guide
├── creds                        # Test VM credentials (gitignored)
├── create/                      # Phase 1: Create all resources
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── terraform.tfvars         # Placeholder values (edit for testing)
├── import/                      # Phase 2: Import all resources
│   ├── main.tf                  # Mirrors create/main.tf exactly
│   ├── variables.tf
│   ├── imports.tf               # Import blocks with literal IDs
│   └── terraform.tfvars         # Placeholder values (edit for testing)
└── modify/                      # Phase 3: Update all resources
    ├── main.tf
    ├── variables.tf
    ├── outputs.tf
    └── terraform.tfvars         # Placeholder values (edit for testing)
```

## Resources Tested

| Resource Type | Create | Import | Modify |
|--------------|--------|--------|--------|
| `trueform_pool` | Creates test pool | Imports by ID | Removed from config (destroyed) |
| `trueform_dataset` | LZ4 compression | Imports by path | Compression → GZIP |
| `trueform_snapshot` | Creates snapshot | Imports by full path | New snapshot (immutable) |
| `trueform_share_smb` | Creates SMB share | Imports by ID | Updates comment |
| `trueform_share_nfs` | 1 network | Imports by ID | Adds networks, sets read-only |
| `trueform_iscsi_portal` | Creates portal | Imports by ID | Updates comment |
| `trueform_iscsi_initiator` | 1 IQN | Imports by ID | Adds second IQN |
| `trueform_iscsi_target` | Creates target | Imports by ID | Updates alias |
| `trueform_iscsi_extent` | 10MB file extent | Imports by ID | Increases to 200MB |
| `trueform_iscsi_targetextent` | LUN 0 | Imports by ID | Changes to LUN 1 |
| `trueform_user` | Creates user | Imports by ID | Updates email, full name |
| `trueform_cronjob` | Disabled, daily | Imports by ID | Enabled, hourly |
| `trueform_static_route` | Creates route | Imports by ID | Updates description |
| `trueform_app` | Deploys app (configures Docker) | Imports by name | Removed (pool destroyed) |

## Prerequisites

### TrueNAS Scale VM

You need a TrueNAS Scale 25.04+ instance. A VM with snapshot capability is recommended for easy state restoration between test runs.

- **CPU**: 2+ cores
- **RAM**: 8GB minimum
- **Boot disk**: 32GB
- **Test disks**: 4x 512MB+ virtual disks (for pool and Docker/app testing)

After installation:
1. Complete the TrueNAS setup wizard
2. Create an API key: **Credentials > API Keys > Add**
3. Note the IP address and API key
4. **Create a VM snapshot** for easy restoration

### Store Credentials

Create a `creds` file (gitignored):

```
IP: 192.168.1.100
API_KEY: 1-YourAPIKeyHere...
```

### Provider Dev Override

Build the provider and configure a dev override:

```bash
cd /path/to/trueform
go build -o terraform-provider-trueform

cat > ~/.terraformrc << 'EOF'
provider_installation {
  dev_overrides {
    "registry.terraform.io/trueform/trueform" = "/path/to/trueform"
  }
  direct {}
}
EOF
```

**Remove the dev_overrides block from `~/.terraformrc` when done testing.**

### Discover Disk Names

Query the TrueNAS API for available disk identifiers:

```bash
curl -k -X POST "https://<HOST>/api/current" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"msg":"method","method":"disk.query","params":[[],{"select":["name","size","type"]}]}'
```

### Configure Variables

Update `terraform.tfvars` in each subdirectory with your TrueNAS connection details and disk names. The committed files contain placeholder values.

## Test Phases

### Phase 1: Create

```bash
cd test-resources/create
terraform apply -auto-approve
```

**Expected**: 15 resources created (includes Docker configuration and app deployment).

Note the resource IDs in the output — you'll need them for the import phase.

### Phase 2: Import

The import config mirrors `create/main.tf` exactly. After import, `terraform plan` should show no changes.

1. Update `import/imports.tf` with correct resource IDs from the create output:
   - Pool ID (usually `1`)
   - User ID (check create output, e.g. `71`)
   - Dataset ID: `testpool/tftest_dataset`
   - Snapshot ID: `testpool/tftest_dataset@tftest_snapshot`
   - All others are typically `1`

   **Note**: Import block `id` values must be literal strings — Terraform does not allow variables in import blocks.

2. Apply and verify:

   ```bash
   cd test-resources/import
   terraform apply -auto-approve   # Imports 14 resources (app values update expected)
   terraform plan                   # Should show "No changes"
   ```

### Phase 3: Modify

1. Copy state from the import directory:

   ```bash
   cp test-resources/import/terraform.tfstate test-resources/modify/terraform.tfstate
   ```

2. Apply:

   ```bash
   cd test-resources/modify
   terraform apply -auto-approve
   ```

   **Expected**: 1 added, 11 changed, 2 destroyed.
   - **1 added**: New snapshot (snapshots are immutable)
   - **11 changed**: Updated resources
   - **2 destroyed**: Old snapshot replaced, pool removed from config (cascades Docker/app teardown)

### Phase 4: Destroy

```bash
cd test-resources/modify
terraform destroy -auto-approve
terraform state list               # Should be empty
```

## Complete Test Cycle

```bash
# Ensure VM is in clean state (restore snapshot if needed)

# Phase 1: Create
cd test-resources/create
terraform apply -auto-approve

# Phase 2: Import
cd ../import
# Update imports.tf IDs if needed
terraform apply -auto-approve
terraform plan                      # Verify: "No changes"

# Phase 3: Modify
cp terraform.tfstate ../modify/terraform.tfstate
cd ../modify
terraform apply -auto-approve

# Phase 4: Destroy
terraform destroy -auto-approve

# Clean up
# - Revert terraform.tfvars files to placeholder values
# - Remove dev_overrides from ~/.terraformrc
# - Optionally restore VM snapshot for next run
```

## Variables Reference

### Required

| Variable | Description | Used In |
|----------|-------------|---------|
| `truenas_host` | TrueNAS host IP or hostname | all |
| `truenas_api_key` | API key for authentication | all |
| `pool_disks` | List of disk identifiers (e.g., `["xvdb", "xvdc"]`) | create, import |
| `nfs_allowed_network` | CIDR for NFS access (e.g., `192.168.1.0/24`) | create, import |
| `nfs_allowed_networks` | List of CIDRs for NFS access | modify |
| `static_route_gateway` | Gateway IP for static route | all |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `truenas_verify_ssl` | `false` | Verify SSL certificates |
| `test_prefix` | `"tftest"` | Prefix for resource names |
| `pool_name` | `"testpool"` | Name of the test pool |
| `iscsi_listen_ip` | `"0.0.0.0"` | iSCSI portal listen address |
| `static_route_destination` | `"10.99.99.0/24"` | Test route destination |
| `test_user_password` | `"TestPassword123!"` | Password for test user |
| `test_app_name` | `"ix-app"` | Catalog app for testing |
| `test_app_train` | `"stable"` | Catalog train for test app |
| `test_app_version` | `"1.3.4"` | Version of test app |

## Troubleshooting

### "Provider produced inconsistent result after apply"

The provider returned different values than expected. Common cause: a field resets to a default when not explicitly included in the update payload (e.g., the SMB `enabled` field on TrueNAS Scale 25). Fix: ensure the provider always sends the field in updates.

### Pool destroy cascades

Destroying a pool on TrueNAS destroys all datasets and snapshots within it. The modify config intentionally omits the pool resource, which causes Terraform to destroy it. This is expected.

### Import "Variables may not be used here"

Terraform import blocks require literal strings for the `id` field. Use hardcoded values, not variable references.

### "Resource already exists"

A resource with the same name exists on TrueNAS. Either destroy it manually or restore the VM snapshot.

### Connection timeout

Verify the VM is running, the IP is correct, and the API key is valid:

```bash
curl -k https://<truenas-ip>/api/current
```

### State file mismatch

If the state doesn't match TrueNAS reality, either restore the VM snapshot or delete the state files and start over.

## Security

**Never commit real credentials.** The following are gitignored:

- `**/creds` — credential files
- `**/*.tfstate*` — Terraform state (may contain secrets)
- `test-resources/terraform.tfvars` — root-level shared variables

The `terraform.tfvars` files in subdirectories are tracked with placeholder values. **Always revert them to placeholders after testing.**
