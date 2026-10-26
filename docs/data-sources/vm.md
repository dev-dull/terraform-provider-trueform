---
page_title: "trueform_vm Data Source - Trueform"
subcategory: "Virtualization"
description: |-
  Retrieves information about an existing virtual machine on TrueNAS.
---

# trueform_vm (Data Source)

!> **Deprecated.** Trueform is no longer maintained. Use the official [`truenas/truenas`](https://registry.terraform.io/providers/truenas/truenas) provider; the [migration guide](https://registry.terraform.io/providers/dev-dull/trueform/latest/docs/guides/migrating-to-truenas-provider) moves existing state without recreating anything.

Retrieves information about an existing virtual machine on TrueNAS Scale.

## Example Usage

```hcl
data "trueform_vm" "webserver" {
  name = "webserver"
}

output "vm_status" {
  value = data.trueform_vm.webserver.status
}

output "vm_memory" {
  value = "${data.trueform_vm.webserver.memory} MB"
}
```

## Schema

### Required

- `name` (String) VM name to look up.

### Read-Only

- `autostart` (Boolean) Whether VM starts automatically on boot.
- `bootloader` (String) Bootloader type.
- `cores` (Number) CPU cores per socket.
- `description` (String) VM description.
- `id` (Number) VM identifier.
- `memory` (Number) Memory in MB.
- `status` (String) VM status.
- `threads` (Number) CPU threads per core.
- `vcpus` (Number) Number of virtual CPUs.
