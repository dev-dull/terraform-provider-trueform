# =============================================================================
# Import Blocks - Import all resources created by the 'create' configuration
# =============================================================================
# Import block IDs must be literal strings (no variables allowed).
# Update these values after running 'terraform apply' in the 'create' directory.
# =============================================================================

# Pool
import {
  to = trueform_pool.test
  id = "4"
}

# Dataset
import {
  to = trueform_dataset.test
  id = "testpool/tftest_dataset"
}

# Snapshot
import {
  to = trueform_snapshot.test
  id = "testpool/tftest_dataset@tftest_snapshot"
}

# SMB Share
import {
  to = trueform_share_smb.test
  id = "4"
}

# NFS Share
import {
  to = trueform_share_nfs.test
  id = "4"
}

# iSCSI Portal
import {
  to = trueform_iscsi_portal.test
  id = "4"
}

# iSCSI Initiator
import {
  to = trueform_iscsi_initiator.test
  id = "4"
}

# iSCSI Target
import {
  to = trueform_iscsi_target.test
  id = "4"
}

# iSCSI Extent
import {
  to = trueform_iscsi_extent.test
  id = "4"
}

# iSCSI Target Extent Mapping
import {
  to = trueform_iscsi_targetextent.test
  id = "4"
}

# User
import {
  to = trueform_user.test
  id = "74"
}

# Cronjob
import {
  to = trueform_cronjob.test
  id = "4"
}

# Static Route
import {
  to = trueform_static_route.test
  id = "4"
}

# App (imported by name)
import {
  to = trueform_app.test
  id = "tftest-app"
}
