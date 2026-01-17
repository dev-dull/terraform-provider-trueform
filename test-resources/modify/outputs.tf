# =============================================================================
# Outputs - Resource IDs and Information
# =============================================================================

output "dataset_id" {
  description = "ID of the modified dataset"
  value       = trueform_dataset.test.id
}

output "dataset_name" {
  description = "Name of the modified dataset"
  value       = trueform_dataset.test.name
}

output "dataset_compression" {
  description = "Compression setting of the modified dataset"
  value       = trueform_dataset.test.compression
}

output "snapshot_id" {
  description = "ID of the new snapshot"
  value       = trueform_snapshot.test.id
}

output "smb_share_id" {
  description = "ID of the modified SMB share"
  value       = trueform_share_smb.test.id
}

output "smb_share_readonly" {
  description = "Read-only status of the modified SMB share"
  value       = trueform_share_smb.test.ro
}

output "nfs_share_id" {
  description = "ID of the modified NFS share"
  value       = trueform_share_nfs.test.id
}

output "nfs_share_networks" {
  description = "Networks allowed for the modified NFS share"
  value       = trueform_share_nfs.test.networks
}

output "iscsi_portal_id" {
  description = "ID of the modified iSCSI portal"
  value       = trueform_iscsi_portal.test.id
}

output "iscsi_initiator_id" {
  description = "ID of the modified iSCSI initiator"
  value       = trueform_iscsi_initiator.test.id
}

output "iscsi_target_id" {
  description = "ID of the modified iSCSI target"
  value       = trueform_iscsi_target.test.id
}

output "iscsi_extent_id" {
  description = "ID of the modified iSCSI extent"
  value       = trueform_iscsi_extent.test.id
}

output "iscsi_targetextent_id" {
  description = "ID of the modified iSCSI target-extent mapping"
  value       = trueform_iscsi_targetextent.test.id
}

output "iscsi_targetextent_lunid" {
  description = "LUN ID of the modified iSCSI target-extent mapping"
  value       = trueform_iscsi_targetextent.test.lunid
}

output "user_id" {
  description = "ID of the modified user"
  value       = trueform_user.test.id
}

output "user_sudo" {
  description = "Sudo status of the modified user"
  value       = trueform_user.test.sudo
}

output "cronjob_id" {
  description = "ID of the modified cronjob"
  value       = trueform_cronjob.test.id
}

output "cronjob_enabled" {
  description = "Enabled status of the modified cronjob"
  value       = trueform_cronjob.test.enabled
}

output "static_route_id" {
  description = "ID of the modified static route"
  value       = trueform_static_route.test.id
}

output "modifications_summary" {
  description = "Summary of modifications made to resources"
  value = {
    dataset = {
      compression = "gzip (was lz4)"
      quota       = "2GB (was 1GB)"
    }
    snapshot = {
      name = "${var.test_prefix}_snapshot_v2 (new snapshot)"
    }
    smb_share = {
      ro         = "true (was false)"
      guestok    = "true (was false)"
      recyclebin = "true (was not set)"
    }
    nfs_share = {
      ro       = "true (was false)"
      networks = "multiple (was single)"
    }
    iscsi_portal = {
      comment = "modified"
    }
    iscsi_initiator = {
      initiators = "2 IQNs (was 1)"
    }
    iscsi_target = {
      alias = "modified"
    }
    iscsi_extent = {
      filesize = "200MB (was 100MB)"
    }
    iscsi_targetextent = {
      lunid = "1 (was 0)"
    }
    user = {
      sudo  = "true (was false)"
      email = "modified"
    }
    cronjob = {
      enabled  = "true (was false)"
      schedule = "hourly (was daily)"
    }
    static_route = {
      description = "modified"
    }
  }
}
