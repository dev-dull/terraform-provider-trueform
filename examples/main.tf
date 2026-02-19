# =============================================================================
# Trueform Provider - Example Configuration
# =============================================================================
# This file demonstrates basic usage of the Trueform Terraform provider
# for managing TrueNAS Scale resources.
#
# For more comprehensive examples, see the test-resources/ directory.
# =============================================================================

terraform {
  required_providers {
    trueform = {
      source = "registry.terraform.io/trueform/trueform"
    }
  }
}

# =============================================================================
# Provider Configuration
# =============================================================================

provider "trueform" {
  host       = var.truenas_host
  api_key    = var.truenas_api_key
  verify_ssl = var.truenas_verify_ssl
}

variable "truenas_host" {
  description = "TrueNAS host address (IP or hostname)"
  type        = string
}

variable "truenas_api_key" {
  description = "TrueNAS API key"
  type        = string
  sensitive   = true
}

variable "truenas_verify_ssl" {
  description = "Verify SSL certificates"
  type        = bool
  default     = true
}

# =============================================================================
# Data Sources
# =============================================================================

# Look up an existing pool
data "trueform_pool" "main" {
  name = "tank"
}

# Look up an existing user
data "trueform_user" "admin" {
  username = "admin"
}

# Look up an existing dataset
data "trueform_dataset" "root" {
  id = "tank"
}

# =============================================================================
# Dataset
# =============================================================================

resource "trueform_dataset" "media" {
  pool        = data.trueform_pool.main.name
  name        = "media"
  compression = "LZ4"
  quota       = 1099511627776 # 1TB
  comments    = "Media storage dataset"
}

# =============================================================================
# SMB Share
# =============================================================================

resource "trueform_share_smb" "media" {
  name      = "media"
  path      = "/mnt/${data.trueform_pool.main.name}/media"
  comment   = "Media share"
  enabled   = true
  browsable = true
  guestok   = false
  ro        = false

  depends_on = [trueform_dataset.media]
}

# =============================================================================
# NFS Share
# =============================================================================

resource "trueform_share_nfs" "media" {
  path    = "/mnt/${data.trueform_pool.main.name}/media"
  enabled = true
  ro      = false

  networks = ["192.168.1.0/24"]

  maproot_user  = "root"
  maproot_group = "wheel"

  depends_on = [trueform_dataset.media]
}

# =============================================================================
# Snapshot
# =============================================================================

resource "trueform_snapshot" "media_daily" {
  dataset   = trueform_dataset.media.id
  name      = "daily-snapshot"
  recursive = false

  depends_on = [trueform_dataset.media]
}

# =============================================================================
# User
# =============================================================================

resource "trueform_user" "media_user" {
  username  = "mediauser"
  full_name = "Media User"
  password  = "changeme123"
  email     = "media@example.com"

  shell = "/usr/sbin/nologin"
  smb   = true
}

# =============================================================================
# Cron Job
# =============================================================================

resource "trueform_cronjob" "backup_script" {
  user        = "root"
  command     = "/usr/local/bin/backup.sh"
  description = "Daily backup script"
  enabled     = true
  stdout      = true
  stderr      = true

  schedule = {
    minute = "0"
    hour   = "2"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}

# =============================================================================
# Static Route
# =============================================================================

resource "trueform_static_route" "internal" {
  destination = "10.0.0.0/8"
  gateway     = "192.168.1.1"
  description = "Route to internal network"
}

# =============================================================================
# iSCSI Configuration
# =============================================================================

# Portal - defines where iSCSI listens
resource "trueform_iscsi_portal" "default" {
  comment = "Default iSCSI portal"

  listen = [
    {
      ip   = "0.0.0.0"
      port = 3260
    }
  ]
}

# Initiator - defines who can connect
resource "trueform_iscsi_initiator" "trusted" {
  comment = "Trusted initiators"

  initiators = [
    "iqn.2024-01.com.example:server1",
    "iqn.2024-01.com.example:server2",
  ]
}

# Target - the iSCSI target name
resource "trueform_iscsi_target" "storage" {
  name  = "storage"
  alias = "Storage Target"
  mode  = "ISCSI"

  groups = [
    {
      portal    = trueform_iscsi_portal.default.id
      initiator = trueform_iscsi_initiator.trusted.id
    }
  ]
}

# Extent (File-based) - the actual storage
resource "trueform_iscsi_extent" "lun0" {
  name     = "storage-lun0"
  type     = "FILE"
  path     = "/mnt/${data.trueform_pool.main.name}/iscsi/lun0.img"
  filesize = 10737418240 # 10GB

  blocksize = 512
  rpm       = "SSD"
}

# Target-Extent mapping - connects target to extent
resource "trueform_iscsi_targetextent" "lun0_mapping" {
  target = trueform_iscsi_target.storage.id
  extent = trueform_iscsi_extent.lun0.id
  lunid  = 0
}

# =============================================================================
# Application (Optional - requires Docker pool configured)
# =============================================================================

# resource "trueform_app" "myapp" {
#   name        = "myapp"
#   catalog_app = "ix-app"
#   train       = "stable"
#   version     = "1.3.4"
#
#   values = jsonencode({
#     image = {
#       repository  = "nginx"
#       tag         = "latest"
#       pull_policy = "missing"
#     }
#   })
# }

# =============================================================================
# Virtual Machine (Optional - requires VM license/capability)
# =============================================================================

# resource "trueform_vm" "ubuntu" {
#   name        = "ubuntu-server"
#   description = "Ubuntu Server VM"
#
#   vcpus   = 2
#   cores   = 1
#   threads = 1
#   memory  = 4096
#
#   bootloader = "UEFI"
#   autostart  = false
# }

# resource "trueform_vm_device" "ubuntu_disk" {
#   vm    = trueform_vm.ubuntu.id
#   dtype = "DISK"
#   order = 1001
#
#   disk_path = "/dev/zvol/${data.trueform_pool.main.name}/vms/ubuntu-boot"
#   disk_type = "VIRTIO"
# }

# resource "trueform_vm_device" "ubuntu_nic" {
#   vm    = trueform_vm.ubuntu.id
#   dtype = "NIC"
#   order = 1002
#
#   nic_type   = "VIRTIO"
#   nic_attach = "br0"
# }

# =============================================================================
# Outputs
# =============================================================================

output "pool_info" {
  description = "Information about the main storage pool"
  value = {
    name   = data.trueform_pool.main.name
    status = data.trueform_pool.main.status
    size   = data.trueform_pool.main.size
    free   = data.trueform_pool.main.free
  }
}

output "dataset_path" {
  description = "Path to the media dataset"
  value       = "/mnt/${trueform_dataset.media.id}"
}

output "smb_share_name" {
  description = "Name of the SMB share"
  value       = trueform_share_smb.media.name
}

output "iscsi_target_name" {
  description = "iSCSI target name"
  value       = trueform_iscsi_target.storage.name
}
