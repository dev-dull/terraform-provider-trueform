# Integration Testing Guide

This guide explains how to perform integration testing of the Trueform Terraform Provider using a real TrueNAS Scale instance.

## Overview

The integration tests verify that the provider correctly manages TrueNAS resources through the complete lifecycle:

1. **Create** - Provision new resources
2. **Read** - Refresh and import existing resources
3. **Update** - Modify resource attributes
4. **Delete** - Clean up resources

## Directory Structure

```
test-resources/
├── INTEGRATION_TESTING.md    # This guide
├── README.md                 # Quick reference
├── creds                     # Test VM credentials (DO NOT COMMIT)
├── terraform.tfvars          # Shared variables (DO NOT COMMIT)
├── create/                   # Phase 1: Create all resources
│   ├── main.tf              # Resource definitions
│   ├── variables.tf         # Variable declarations
│   ├── outputs.tf           # Output values
│   ├── terraform.tfvars     # Test values (DO NOT COMMIT)
│   └── terraform.tfvars.example
└── modify/                   # Phase 2: Update resources
    ├── main.tf              # Modified resource definitions
    ├── variables.tf         # Variable declarations
    ├── outputs.tf           # Output values
    ├── terraform.tfvars     # Test values (DO NOT COMMIT)
    └── terraform.tfvars.example
```

## Prerequisites

### 1. TrueNAS Scale Instance

You need a TrueNAS Scale 25.04+ instance for testing. Options:

| Option | Pros | Cons |
|--------|------|------|
| **VM (Recommended)** | Isolated, snapshot/restore, safe | Requires hypervisor |
| **Dedicated hardware** | Real-world testing | Risk of data loss |
| **Existing NAS** | No setup needed | Risk to production data |

**Recommended**: Use a VM with snapshot capability for easy state restoration between test runs.

### 2. VM Setup (Recommended)

Create a TrueNAS Scale VM with:

- **CPU**: 2+ cores
- **RAM**: 8GB minimum
- **Boot disk**: 32GB
- **Test disks**: 4x 256MB virtual disks (for pool testing)

After installation:
1. Complete the TrueNAS setup wizard
2. Create an API key: **Credentials > API Keys > Add**
3. Note the IP address and API key
4. **Create a VM snapshot** for easy restoration

### 3. Store Test Credentials

Create a `creds` file (gitignored):

```
IP: 192.168.1.195
USER: truenas_admin
API_KEY: 1-YourAPIKeyHere...
```

### 4. Provider Development Setup

Ensure the provider is built and configured for local development:

```bash
# Build the provider
cd /path/to/trueform
go build -o terraform-provider-trueform

# Create dev override config
mkdir -p ~/.terraform.d
cat > ~/.terraformrc << 'EOF'
provider_installation {
  dev_overrides {
    "registry.terraform.io/trueform/trueform" = "/path/to/trueform"
  }
  direct {}
}
EOF
```

## Test Phases

### Phase 1: Create Resources

The `create/` directory provisions one instance of each supported resource type.

#### Resources Created

| Resource | Name | Description |
|----------|------|-------------|
| `trueform_pool` | testpool | ZFS pool from test disks |
| `trueform_dataset` | tftest_dataset | Dataset with LZ4 compression |
| `trueform_snapshot` | tftest_snapshot | Snapshot of the dataset |
| `trueform_share_smb` | tftest_smb | SMB/CIFS share |
| `trueform_share_nfs` | tftest_nfs | NFS export |
| `trueform_iscsi_portal` | - | iSCSI portal on port 3260 |
| `trueform_iscsi_initiator` | - | iSCSI initiator group |
| `trueform_iscsi_target` | tftest-target | iSCSI target |
| `trueform_iscsi_extent` | tftest-extent | 10MB file-based extent |
| `trueform_iscsi_targetextent` | - | LUN 0 mapping |
| `trueform_user` | tftest_user | Local user account |
| `trueform_cronjob` | - | Disabled daily cron job |
| `trueform_static_route` | - | Static network route |

#### Running Create Tests

```bash
cd test-resources/create

# Configure variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your TrueNAS details

# Initialize (skip if using dev overrides)
terraform init

# Preview changes
terraform plan

# Apply changes
terraform apply
```

#### Expected Output

```
Apply complete! Resources: 13 added, 0 changed, 0 destroyed.

Outputs:
cronjob_id = 1
dataset_id = "testpool/tftest_dataset"
...
```

### Phase 2: Modify Resources

The `modify/` directory updates the resources created in Phase 1 to test update operations.

#### Modifications Applied

| Resource | Change |
|----------|--------|
| Dataset | Compression: LZ4 → GZIP |
| Snapshot | New snapshot (immutable resource) |
| SMB Share | Updated comment |
| NFS Share | Added networks, set read-only |
| iSCSI Portal | Updated comment |
| iSCSI Initiator | Added second IQN |
| iSCSI Target | Updated alias |
| iSCSI Extent | Filesize: 10MB → 200MB |
| iSCSI TargetExtent | LUN: 0 → 1 |
| User | Updated email, full name |
| Cronjob | Enabled, schedule: daily → hourly |
| Static Route | Updated description |

#### Running Modify Tests

```bash
cd test-resources/modify

# Configure variables
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with matching values

# Copy state from create phase
cp ../create/terraform.tfstate .

# Preview changes (should show updates, not creates)
terraform plan

# Apply modifications
terraform apply
```

#### Expected Output

```
Apply complete! Resources: 1 added, 11 changed, 2 destroyed.
```

- **1 added**: New snapshot (immutable)
- **11 changed**: Updated resources
- **2 destroyed**: Old snapshot, pool (removed from config)

### Phase 3: Destroy Resources

Clean up all test resources:

```bash
cd test-resources/modify  # or create, depending on current state
terraform destroy
```

## Complete Test Cycle

Run a full create → modify → destroy cycle:

```bash
# Ensure VM is in clean state (restore snapshot if needed)

# Phase 1: Create
cd test-resources/create
rm -rf .terraform* terraform.tfstate*
terraform init
terraform apply -auto-approve

# Phase 2: Modify
cd ../modify
rm -rf .terraform* terraform.tfstate*
cp ../create/terraform.tfstate .
terraform init
terraform apply -auto-approve

# Phase 3: Destroy
terraform destroy -auto-approve

# Restore VM snapshot for next test run
```

## Testing Specific Resources

To test a specific resource type in isolation:

1. Comment out other resources in `main.tf`
2. Run `terraform apply`
3. Make changes and run `terraform apply` again
4. Run `terraform destroy`

Example - testing only the dataset resource:

```bash
cd test-resources/create

# Edit main.tf to comment out everything except:
# - provider block
# - locals block
# - trueform_pool.test (required for dataset)
# - trueform_dataset.test

terraform apply
# Make changes...
terraform apply
terraform destroy
```

## Resetting Test Environment

### Option 1: VM Snapshot Restore (Recommended)

Restore the VM to its initial snapshot state. This provides a completely clean environment.

### Option 2: Terraform Destroy

```bash
cd test-resources/modify  # or wherever state file is
terraform destroy -auto-approve
```

### Option 3: Manual Cleanup

If Terraform state is corrupted:

1. Log into TrueNAS web UI
2. Delete resources with `tftest` prefix
3. Remove local state files:
   ```bash
   rm -rf test-resources/*/terraform.tfstate*
   rm -rf test-resources/*/.terraform*
   ```

## Troubleshooting

### "Provider produced inconsistent result after apply"

The provider is returning different values than expected. Common causes:

- Field defaults differ between TrueNAS versions
- Computed fields not properly handled

**Fix**: Check the resource's `Read` function handles all field variations.

### "Resource already exists"

A resource with the same name exists on TrueNAS.

**Fix**: Either destroy the existing resource or restore the VM snapshot.

### "InstanceNotFound" or "does not exist"

The resource was deleted outside of Terraform.

**Fix**: Remove from state with `terraform state rm <resource>` or restore VM snapshot.

### Connection timeout

TrueNAS is unreachable.

**Fix**: Verify:
- VM is running
- IP address is correct
- API is enabled in TrueNAS settings
- No firewall blocking port 443

### State file mismatch

The state file doesn't match the actual TrueNAS state.

**Fix**: Either:
- Restore VM to match state
- Delete state and re-import resources
- Use `terraform refresh` to update state

## Tips for Development Testing

### 1. Use VM Snapshots Liberally

Create snapshots at key points:
- After initial TrueNAS setup (clean state)
- After successful create phase
- Before testing destructive operations

### 2. Enable Debug Logging

```bash
export TF_LOG=DEBUG
terraform apply
```

### 3. Test One Resource at a Time

When debugging, isolate the problematic resource by commenting out others.

### 4. Check TrueNAS API Directly

Use the TrueNAS web UI or API explorer to verify resource state:
- Web UI: `https://<truenas-ip>/ui`
- API Docs: `https://<truenas-ip>/api/docs`

### 5. Compare Plan vs Apply

If apply produces different results than plan showed:
1. Check for computed fields not in schema
2. Verify default values match TrueNAS defaults
3. Look for case sensitivity issues (TrueNAS often uses uppercase)

## Security Notes

**Never commit sensitive files:**
- `terraform.tfvars` (contains API keys)
- `creds` (contains credentials)
- `terraform.tfstate` (may contain secrets)

These patterns are already in `.gitignore`:
```
*.tfvars
!*.tfvars.example
*.tfstate
*.tfstate.*
creds
```
