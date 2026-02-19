# =============================================================================
# Outputs - Resource IDs and Information
# =============================================================================

output "dataset_id" {
  description = "ID of the created dataset"
  value       = trueform_dataset.test.id
}

output "dataset_name" {
  description = "Full path of the created dataset"
  value       = trueform_dataset.test.id
}

output "snapshot_id" {
  description = "ID of the created snapshot"
  value       = trueform_snapshot.test.id
}

output "smb_share_id" {
  description = "ID of the created SMB share"
  value       = trueform_share_smb.test.id
}

output "nfs_share_id" {
  description = "ID of the created NFS share"
  value       = trueform_share_nfs.test.id
}

output "iscsi_portal_id" {
  description = "ID of the created iSCSI portal"
  value       = trueform_iscsi_portal.test.id
}

output "iscsi_initiator_id" {
  description = "ID of the created iSCSI initiator"
  value       = trueform_iscsi_initiator.test.id
}

output "iscsi_target_id" {
  description = "ID of the created iSCSI target"
  value       = trueform_iscsi_target.test.id
}

output "iscsi_extent_id" {
  description = "ID of the created iSCSI extent"
  value       = trueform_iscsi_extent.test.id
}

output "iscsi_targetextent_id" {
  description = "ID of the created iSCSI target-extent mapping"
  value       = trueform_iscsi_targetextent.test.id
}

output "user_id" {
  description = "ID of the created user"
  value       = trueform_user.test.id
}

output "user_uid" {
  description = "UID of the created user"
  value       = trueform_user.test.uid
}

output "cronjob_id" {
  description = "ID of the created cronjob"
  value       = trueform_cronjob.test.id
}

output "static_route_id" {
  description = "ID of the created static route"
  value       = trueform_static_route.test.id
}

output "docker_status" {
  description = "Status of the Docker service"
  value       = trueform_service_docker.config.status
}

output "app_id" {
  description = "ID of the created app"
  value       = trueform_app.test.id
}

output "app_state" {
  description = "State of the created app"
  value       = trueform_app.test.state
}

output "summary" {
  description = "Summary of all created resources"
  value = {
    dataset            = trueform_dataset.test.id
    snapshot           = trueform_snapshot.test.id
    smb_share          = trueform_share_smb.test.name
    nfs_share_path     = trueform_share_nfs.test.path
    iscsi_portal       = trueform_iscsi_portal.test.id
    iscsi_initiator    = trueform_iscsi_initiator.test.id
    iscsi_target       = trueform_iscsi_target.test.name
    iscsi_extent       = trueform_iscsi_extent.test.name
    iscsi_targetextent = trueform_iscsi_targetextent.test.id
    user               = trueform_user.test.username
    cronjob            = trueform_cronjob.test.description
    static_route       = trueform_static_route.test.destination
    docker             = trueform_service_docker.config.status
    app                = trueform_app.test.name
  }
}
