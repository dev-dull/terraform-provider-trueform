# =============================================================================
# Trueform Provider Test Suite - Resource Creation
# =============================================================================
# This configuration creates one of each resource type for testing purposes.
# Run `terraform apply` to create all resources, then switch to the modify
# directory to test update operations.
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
  share_path   = "/mnt/${var.pool_name}/${var.test_prefix}_dataset"
}

# =============================================================================
# Pool
# =============================================================================

resource "trueform_pool" "test" {
  name = var.pool_name

  topology = [
    {
      type  = "data"
      disks = var.pool_disks
    }
  ]
}

# =============================================================================
# Dataset
# =============================================================================

resource "trueform_dataset" "test" {
  pool        = trueform_pool.test.name
  name        = local.dataset_name
  comments    = "Test dataset created by Terraform provider test suite"
  compression = "LZ4"
  atime       = "OFF"
  # Note: quota must be >= 1GB or omitted for unlimited

  depends_on = [trueform_pool.test]
}

# =============================================================================
# Snapshot
# =============================================================================

resource "trueform_snapshot" "test" {
  dataset   = trueform_dataset.test.id
  name      = "${var.test_prefix}_snapshot"
  recursive = false

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# SMB Share
# =============================================================================

resource "trueform_share_smb" "test" {
  name      = "${var.test_prefix}_smb"
  path      = local.share_path
  enabled   = true
  browsable = true
  ro        = false
  guestok   = false
  comment   = "Test SMB share created by Terraform provider test suite"

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# NFS Share
# =============================================================================

resource "trueform_share_nfs" "test" {
  path     = local.share_path
  enabled  = true
  ro       = false
  networks = [var.nfs_allowed_network]
  comment  = "Test NFS share created by Terraform provider test suite"

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# iSCSI Portal
# =============================================================================

resource "trueform_iscsi_portal" "test" {
  comment = "${var.test_prefix} iSCSI Portal"
  listen = [
    {
      ip = var.iscsi_listen_ip
    }
  ]
}

# =============================================================================
# iSCSI Initiator
# =============================================================================

resource "trueform_iscsi_initiator" "test" {
  comment    = "${var.test_prefix} iSCSI Initiator"
  initiators = ["iqn.2024-01.com.example:${var.test_prefix}-initiator"]
}

# =============================================================================
# iSCSI Target
# =============================================================================

resource "trueform_iscsi_target" "test" {
  name  = "${var.test_prefix}-target"
  alias = "Test iSCSI Target"
}

# =============================================================================
# iSCSI Extent (file-based for testing)
# =============================================================================

resource "trueform_iscsi_extent" "test" {
  name     = "${var.test_prefix}-extent"
  type     = "FILE"
  path     = "${local.share_path}/${var.test_prefix}_extent.img"
  filesize = 10485760 # 10 MB (small disks)
  comment  = "Test iSCSI extent created by Terraform provider test suite"

  depends_on = [trueform_dataset.test]
}

# =============================================================================
# iSCSI Target Extent Mapping
# =============================================================================

resource "trueform_iscsi_targetextent" "test" {
  target = trueform_iscsi_target.test.id
  extent = trueform_iscsi_extent.test.id
  lunid  = 0
}

# =============================================================================
# User
# =============================================================================

resource "trueform_user" "test" {
  username  = "${var.test_prefix}_user"
  full_name = "Test User"
  password  = var.test_user_password
  email     = "${var.test_prefix}@example.com"
  shell     = "/usr/sbin/nologin"
  smb       = true
  locked    = false
}

# =============================================================================
# Cronjob
# =============================================================================

resource "trueform_cronjob" "test" {
  user        = "root"
  command     = "echo 'Terraform provider test cronjob executed at $(date)' >> /tmp/${var.test_prefix}_cronjob.log"
  description = "Test cronjob created by Terraform provider test suite"
  enabled     = false # Disabled by default for safety
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
  stdout = true
  stderr = true
}

# =============================================================================
# Static Route
# =============================================================================

resource "trueform_static_route" "test" {
  destination = var.static_route_destination
  gateway     = var.static_route_gateway
  description = "Test static route created by Terraform provider test suite"
}

# =============================================================================
# Docker Configuration
# =============================================================================
# Apps require Docker to be configured with a pool. This uses the TrueNAS API
# to set the apps pool after the pool is created, before the app is deployed.
# =============================================================================

resource "terraform_data" "docker_config" {
  input = trueform_pool.test.name

  provisioner "local-exec" {
    command = <<-EOT
      # Configure Docker to use the test pool via TrueNAS REST API
      JOB_ID=$(curl -sk -X PUT "https://${var.truenas_host}/api/v2.0/docker" \
        -H "Authorization: Bearer ${var.truenas_api_key}" \
        -H "Content-Type: application/json" \
        -d '{"pool": "${trueform_pool.test.name}"}')

      echo "Docker config job $JOB_ID started, waiting..."

      # Poll until the job completes (up to 5 minutes)
      for i in $(seq 1 60); do
        sleep 5
        STATE=$(curl -sk "https://${var.truenas_host}/api/v2.0/core/get_jobs?id=$JOB_ID" \
          -H "Authorization: Bearer ${var.truenas_api_key}" \
          | python3 -c "import sys,json; jobs=json.load(sys.stdin); print(jobs[0]['state'] if jobs else 'UNKNOWN')")

        if [ "$STATE" = "SUCCESS" ]; then
          echo "Docker configured successfully"

          # Wait for Docker service to fully start
          echo "Waiting for Docker service to be ready..."
          for j in $(seq 1 24); do
            DOCKER_STATE=$(curl -sk "https://${var.truenas_host}/api/v2.0/docker/status" \
              -H "Authorization: Bearer ${var.truenas_api_key}" \
              | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','UNKNOWN'))" 2>/dev/null)
            echo "  Docker status: $DOCKER_STATE"
            if [ "$DOCKER_STATE" = "RUNNING" ]; then
              echo "Docker is ready"
              exit 0
            fi
            sleep 5
          done
          echo "Docker configured but service may not be fully ready yet"
          exit 0
        elif [ "$STATE" = "FAILED" ]; then
          echo "ERROR: Docker config job failed"
          exit 1
        fi
        echo "  Waiting... ($STATE)"
      done

      echo "ERROR: Timed out waiting for Docker config"
      exit 1
    EOT
  }

  depends_on = [trueform_pool.test]
}

# =============================================================================
# App
# =============================================================================

resource "trueform_app" "test" {
  name        = "${var.test_prefix}-app"
  catalog_app = var.test_app_name
  train       = var.test_app_train
  version     = var.test_app_version
  values = jsonencode({
    image = {
      repository = "busybox"
      tag        = "latest"
      pull_policy = "missing"
    }
    command = ["sleep", "3600"]
  })

  depends_on = [terraform_data.docker_config]
}
