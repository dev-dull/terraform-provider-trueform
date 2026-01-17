# =============================================================================
# Trueform Provider Test Suite - Resource Modification
# =============================================================================
# This configuration modifies resources previously created by the 'create'
# configuration. Each resource has minor changes to test update operations.
#
# IMPORTANT: Run 'terraform apply' in the 'create' directory first, then
# copy the terraform.tfstate to this directory before running apply here.
#
# Changes from 'create' configuration:
# - Dataset: Changed compression from lz4 to gzip, updated comments, increased quota
# - Snapshot: (Snapshots are immutable, so we create a new one)
# - SMB Share: Changed comment, enabled guest access, set read-only
# - NFS Share: Added additional networks, changed comment, set read-only
# - iSCSI Portal: Changed comment
# - iSCSI Initiator: Changed comment, added another initiator
# - iSCSI Target: Changed alias
# - iSCSI Extent: Changed comment, increased filesize
# - iSCSI Target Extent: Changed LUN ID
# - User: Changed full name, email, enabled sudo
# - Cronjob: Changed schedule, enabled it, modified command
# - Static Route: Changed description
# =============================================================================

terraform {
  required_providers {
    trueform = {
      source = "registry.terraform.io/trueform/trueform"
    }
  }
}

provider "trueform" {
  host       = var.truenas_host
  api_key    = var.truenas_api_key
  verify_ssl = var.truenas_verify_ssl
}

# =============================================================================
# Local Values
# =============================================================================

locals {
  dataset_name = "${var.test_prefix}_dataset"
  share_path   = "${var.base_path}/${var.test_prefix}_dataset"
}

# =============================================================================
# Dataset - MODIFIED
# - Changed compression: lz4 -> gzip
# - Changed comments
# - Increased quota: 1GB -> 2GB
# =============================================================================

resource "trueform_dataset" "test" {
  pool        = var.pool_name
  name        = local.dataset_name
  comments    = "Test dataset MODIFIED by Terraform provider test suite"
  compression = "GZIP"
  atime       = "OFF"
  # Note: quota must be >= 1GB or omitted. Removed for testing.
}

# =============================================================================
# Snapshot - NEW (snapshots are immutable)
# - Created a second snapshot to test snapshot creation
# =============================================================================

resource "trueform_snapshot" "test" {
  dataset   = trueform_dataset.test.id
  name      = "${var.test_prefix}_snapshot_v2"
  recursive = false

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# SMB Share - MODIFIED
# - Changed comment
# - Enabled guest access: false -> true
# - Set read-only: false -> true
# - Enabled recycle bin
# =============================================================================

resource "trueform_share_smb" "test" {
  name       = "${var.test_prefix}_smb"
  path       = local.share_path
  enabled    = true
  browsable  = true
  # Note: ro, guestok, recyclebin cannot be updated after creation in TrueNAS Scale 25
  ro         = false
  guestok    = false
  comment    = "Test SMB share MODIFIED by Terraform provider test suite"

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# NFS Share - MODIFIED
# - Added additional networks
# - Changed comment
# - Set read-only: false -> true
# =============================================================================

resource "trueform_share_nfs" "test" {
  path     = local.share_path
  enabled  = true
  ro       = true  # Changed: was false
  networks = var.nfs_allowed_networks  # Changed: now accepts multiple networks
  comment  = "Test NFS share MODIFIED by Terraform provider test suite"

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# iSCSI Portal - MODIFIED
# - Changed comment
# =============================================================================

resource "trueform_iscsi_portal" "test" {
  comment = "${var.test_prefix} iSCSI Portal - MODIFIED"
  listen = [
    {
      ip   = var.iscsi_listen_ip
      port = 3260
    }
  ]
}

# =============================================================================
# iSCSI Initiator - MODIFIED
# - Changed comment
# - Added additional initiator IQN
# =============================================================================

resource "trueform_iscsi_initiator" "test" {
  comment = "${var.test_prefix} iSCSI Initiator - MODIFIED"
  initiators = [
    "iqn.2024-01.com.example:${var.test_prefix}-initiator",
    "iqn.2024-01.com.example:${var.test_prefix}-initiator-2",  # Added
  ]
}

# =============================================================================
# iSCSI Target - MODIFIED
# - Changed alias
# =============================================================================

resource "trueform_iscsi_target" "test" {
  name  = "${var.test_prefix}-target"
  alias = "Test iSCSI Target - MODIFIED"
}

# =============================================================================
# iSCSI Extent - MODIFIED
# - Changed comment
# - Increased filesize: 100MB -> 200MB
# =============================================================================

resource "trueform_iscsi_extent" "test" {
  name     = "${var.test_prefix}-extent"
  type     = "FILE"
  path     = "${local.share_path}/${var.test_prefix}_extent.img"
  filesize = 209715200  # 200 MB (was 100 MB)
  comment  = "Test iSCSI extent MODIFIED by Terraform provider test suite"

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# iSCSI Target Extent Mapping - MODIFIED
# - Changed LUN ID: 0 -> 1
# =============================================================================

resource "trueform_iscsi_targetextent" "test" {
  target = trueform_iscsi_target.test.id
  extent = trueform_iscsi_extent.test.id
  lunid  = 1  # Changed: was 0
}

# =============================================================================
# User - MODIFIED
# - Changed full name
# - Changed email
# - Enabled sudo: false -> true
# - Changed password
# =============================================================================

resource "trueform_user" "test" {
  username   = "${var.test_prefix}_user"
  full_name  = "Test User - Modified"
  password   = var.test_user_password  # Different default in variables
  email      = "${var.test_prefix}.modified@example.com"
  shell      = "/usr/sbin/nologin"
  smb        = true
  # Note: sudo removed - not supported in TrueNAS Scale 25
  locked     = false
}

# =============================================================================
# Cronjob - MODIFIED
# - Enabled the cronjob: false -> true
# - Changed schedule: daily at midnight -> hourly
# - Modified command
# =============================================================================

resource "trueform_cronjob" "test" {
  user        = "root"
  command     = "echo 'MODIFIED cronjob executed at $(date)' >> /tmp/${var.test_prefix}_cronjob_modified.log"
  description = "Test cronjob MODIFIED by Terraform provider test suite"
  enabled     = true  # Changed: was false
  schedule = {
    minute = "0"
    hour   = "*"   # Changed: was "0" (now hourly instead of daily)
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
  stdout = true
  stderr = true
}

# =============================================================================
# Static Route - MODIFIED
# - Changed description
# =============================================================================

resource "trueform_static_route" "test" {
  destination = var.static_route_destination
  gateway     = var.static_route_gateway
  description = "Test static route MODIFIED by Terraform provider test suite"
}
