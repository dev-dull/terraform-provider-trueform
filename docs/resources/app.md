---
page_title: "trueform_app Resource - Trueform"
subcategory: "Applications"
description: |-
  Manages an application on TrueNAS.
---

# trueform_app (Resource)

Manages an application on TrueNAS Scale. Apps are deployed from the TrueNAS app catalog.

~> **Note:** Changing the `name` or `catalog_app` will force recreation of the application.

~> **Prerequisite:** A Docker/Apps pool must be configured on TrueNAS before deploying apps. Use the [`trueform_service_docker`](service_docker.md) resource, or configure manually in the TrueNAS UI under **Apps > Settings > Choose a pool**.

## Example Usage

### Basic App

```hcl
resource "trueform_app" "plex" {
  name        = "plex"
  catalog_app = "plex"
}
```

### App with Version and Train

```hcl
resource "trueform_app" "nextcloud" {
  name        = "nextcloud"
  catalog_app = "nextcloud"
  train       = "stable"
  version     = "29.0.0"
}
```

### Custom App (ix-app)

```hcl
resource "trueform_app" "myapp" {
  name        = "myapp"
  catalog_app = "ix-app"
  train       = "stable"
  version     = "1.3.5"

  values = jsonencode({
    image = {
      repository  = "nginx"
      tag         = "latest"
      pull_policy = "missing"
    }
  })
}
```

### App with Custom Configuration

```hcl
resource "trueform_app" "minio" {
  name        = "minio"
  catalog_app = "minio"

  values = jsonencode({
    minioStorage = [{
      hostPath       = "/mnt/tank/minio"
      mountPath      = "/data"
      readOnly       = false
    }]
  })
}
```

## Schema

### Required

- `catalog_app` (String) The catalog app to deploy (e.g., `plex`, `nextcloud`). Cannot be changed after creation.
- `name` (String) The name of the app instance. Cannot be changed after creation.

### Optional

- `train` (String) The catalog train (e.g., `stable`, `community`).
- `values` (String) JSON-encoded configuration values for the app. This field is write-only — values are sent to TrueNAS on create/update but cannot be read back, so import will always show a diff if values are set.
- `version` (String) The app version to deploy. Changing this triggers an upgrade.

### Read-Only

- `id` (String) The unique identifier for the app (same as name).
- `metadata` (Map of String) App metadata.
- `state` (String) Current state of the app (e.g., `RUNNING`, `STOPPED`).

## Destroy Behavior

When an app is destroyed, the provider:

1. Stops the app if it is currently running (best-effort — if Docker is unavailable, e.g. due to pool destruction, this is skipped)
2. Deletes the app with `remove_images` and `remove_ix_volumes` enabled
3. If the delete fails (e.g. due to invalid YAML in a custom app), retries with `force_remove_custom_app`
4. If the app has already been removed (e.g. by a cascading pool destroy), the delete is treated as a no-op

## Import

Apps can be imported using the app name:

```shell
terraform import trueform_app.plex plex
```
