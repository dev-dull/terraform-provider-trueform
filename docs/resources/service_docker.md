---
page_title: "trueform_service_docker Resource - Trueform"
subcategory: "Services"
description: |-
  Manages the Docker/Apps service configuration on TrueNAS.
---

# trueform_service_docker (Resource)

Manages the Docker/Apps service configuration on TrueNAS Scale. A pool must be configured for Docker before applications can be deployed using the [`trueform_app`](app.md) resource.

This is a singleton resource — only one Docker service configuration exists per TrueNAS system.

~> **Note:** Destroying this resource unconfigures Docker by removing the pool assignment. This will stop all running applications.

## Example Usage

### Basic Configuration

```hcl
resource "trueform_service_docker" "apps" {
  pool = "tank"
}
```

### With GPU Support

```hcl
resource "trueform_service_docker" "apps" {
  pool   = "tank"
  nvidia = true
}
```

### With App Dependency

```hcl
resource "trueform_pool" "main" {
  name = "tank"
  topology = [
    {
      type  = "data"
      disks = ["sda", "sdb"]
    }
  ]
}

resource "trueform_service_docker" "apps" {
  pool       = trueform_pool.main.name
  depends_on = [trueform_pool.main]
}

resource "trueform_app" "myapp" {
  name        = "myapp"
  catalog_app = "ix-app"
  depends_on  = [trueform_service_docker.apps]
}
```

## Schema

### Required

- `pool` (String) The storage pool to use for Docker/Apps data.

### Optional

- `enable_image_updates` (Boolean) Automatically check for Docker image updates. Defaults to `true`.
- `nvidia` (Boolean) Enable NVIDIA GPU support for containers. Defaults to `false`.

### Read-Only

- `id` (String) Resource identifier (always `docker`).
- `status` (String) Current Docker service status (e.g., `RUNNING`, `INITIALIZING`, `STOPPED`).

## Import

The Docker service can be imported using the literal ID `docker`:

```shell
terraform import trueform_service_docker.apps docker
```
