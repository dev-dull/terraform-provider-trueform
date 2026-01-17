# Trueform Provider Test Suite

This directory contains Terraform configurations to test all resource types in the Trueform provider.

## Directory Structure

```
test-resources/
├── README.md           # This file
├── create/             # Creates one of each resource type
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── terraform.tfvars.example
└── modify/             # Modifies each resource (tests update operations)
    ├── main.tf
    ├── variables.tf
    ├── outputs.tf
    └── terraform.tfvars.example
```

## Resources Tested

| Resource Type | Create Test | Modify Test |
|--------------|-------------|-------------|
| `trueform_dataset` | Creates dataset with lz4 compression | Changes to gzip, increases quota |
| `trueform_snapshot` | Creates snapshot | Creates new snapshot (immutable) |
| `trueform_share_smb` | Creates SMB share | Enables guest access, read-only |
| `trueform_share_nfs` | Creates NFS share with 1 network | Adds networks, sets read-only |
| `trueform_iscsi_portal` | Creates iSCSI portal | Updates comment |
| `trueform_iscsi_initiator` | Creates initiator with 1 IQN | Adds second IQN |
| `trueform_iscsi_target` | Creates iSCSI target | Updates alias |
| `trueform_iscsi_extent` | Creates 100MB file extent | Increases to 200MB |
| `trueform_iscsi_targetextent` | Maps target to extent, LUN 0 | Changes to LUN 1 |
| `trueform_user` | Creates test user | Enables sudo, updates email |
| `trueform_cronjob` | Creates disabled daily cronjob | Enables, changes to hourly |
| `trueform_static_route` | Creates static route | Updates description |

## Prerequisites

1. A running TrueNAS Scale 25.04+ instance
2. An API key with appropriate permissions
3. An existing storage pool
4. The Trueform provider installed (via dev_overrides or registry)

## Usage

### Step 1: Configure Variables

```bash
cd create/
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your TrueNAS connection details
```

### Step 2: Create Resources

```bash
cd create/
terraform init
terraform plan
terraform apply
```

### Step 3: Test Modifications

```bash
cd ../modify/
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with matching values

# Copy state from create directory
cp ../create/terraform.tfstate .

terraform plan    # Should show updates, not creates
terraform apply   # Apply modifications
```

### Step 4: Clean Up

```bash
terraform destroy
```

## Variables Reference

### Required Variables

| Variable | Description |
|----------|-------------|
| `truenas_host` | TrueNAS host IP or hostname |
| `truenas_api_key` | API key for authentication |
| `pool_name` | Name of existing pool (e.g., "tank") |
| `base_path` | Base path for resources (e.g., "/mnt/tank") |
| `nfs_allowed_network` | CIDR for NFS access (create only) |
| `nfs_allowed_networks` | List of CIDRs for NFS (modify only) |
| `static_route_gateway` | Gateway IP for static route |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `truenas_verify_ssl` | `false` | Verify SSL certificates |
| `test_prefix` | `"tftest"` | Prefix for resource names |
| `iscsi_listen_ip` | `"0.0.0.0"` | iSCSI portal listen address |
| `static_route_destination` | `"10.99.99.0/24"` | Test route destination |
| `test_user_password` | varies | Password for test user |

## Notes

- The test prefix (`tftest` by default) is used for all resource names to avoid conflicts
- The cronjob is disabled by default in the create configuration for safety
- Snapshots are immutable, so the modify configuration creates a new snapshot
- Remember to destroy resources when done testing to clean up

## Troubleshooting

### "Resource already exists" errors
Ensure you're using the same `test_prefix` and that no conflicting resources exist.

### State mismatch between create and modify
Copy `terraform.tfstate` from `create/` to `modify/` before running apply in modify.

### Connection timeouts
Check that the TrueNAS host is reachable and the API is enabled.
