---
page_title: "Migrating to the official TrueNAS provider"
subcategory: "Guides"
description: |-
  How to move an existing trueform configuration and its state to the official truenas/truenas provider without recreating anything on your NAS.
---

<!--
Maintainer note: written from a code reading of truenas/terraform-provider-truenas
v1.5.11 (commit 9311a47, 2026-10-06), then exercised end to end on 2026-10-10 against
TrueNAS 26.0.0-BETA.3 with Terraform 1.14.8: pool, dataset, snapshot, SMB, NFS, user,
cronjob, static route, docker config, app, certificate and all five iSCSI resources
migrated with "16 to import, 0 to add, 0 to destroy" and a clean plan afterwards.
The replacement traps, the SMB purpose fallback and the rule 3 overwrite were
reproduced live. vm and vm_device were not tested (no nested virtualization), and
statements still tagged VERIFY were not exercised.
-->

# Migrating to the official TrueNAS provider

iXsystems now publishes an official provider, [`truenas/truenas`](https://registry.terraform.io/providers/truenas/truenas). It covers everything trueform does and much more: 85 resources, including zvols, groups, periodic snapshot tasks, replication, services and ACLs. It is the better long-term home for your configuration, and this guide moves you onto it **without destroying or recreating anything on the NAS**.

## How the migration works

The two providers have different resource types and schemas. The official provider cannot adopt trueform state directly: it has no `moved` support from other providers. Instead, for every trueform resource you:

1. add a **`removed`** block with `destroy = false`, so Terraform *forgets* the trueform resource rather than deleting it; and
2. add an **`import`** block, so the official provider adopts the same object by its ID.

Both happen in a single `terraform apply`. Nothing is created or deleted on the NAS. Only Terraform's bookkeeping changes.

~> **Read the plan before you apply.** If a trueform block is deleted *without* a matching `removed` block, Terraform will destroy the real object. For a pool, that means the pool is exported and its disks are wiped. If an imported resource's config doesn't match what the official provider reads back on certain attributes, Terraform plans a *replace*, which deletes the object. The only acceptable migration plan is **`N to import, 0 to add, 0 to destroy`**, ideally with `0 to change`.

## Before you start

| Requirement | Notes |
|---|---|
| **TrueNAS 25.10 or 26.0** | The official provider is tested on 25.10 and 26.0. 25.04 can connect but is untested, so upgrade TrueNAS first if you are on 25.04. |
| **Terraform ≥ 1.11** | Needed for write-only attributes (e.g. `truenas_user.password`). 1.14+ unlocks `import { identity = ... }`, `terraform query` and Actions, none of which this guide needs. |
| **A state backup** | `terraform state pull > pre-migration.tfstate` |
| **A TrueNAS config backup** | *System → General → Manage Configuration → Download File*. Optionally, take a recursive snapshot of your pools. |
| **No pending changes** | `terraform plan` with trueform should show *No changes*. Don't mix real changes into the migration. |

Migrate a copy of your configuration on a branch. If you have many resources, migrate a few at a time: the procedure works resource by resource.

## Step 1: Switch the provider block

Keep **both** providers declared and configured until the migration apply completes. Terraform still needs trueform to process the `removed` blocks.

```hcl
terraform {
  required_providers {
    trueform = {
      source = "dev-dull/trueform" # your existing trueform entry, unchanged
    }
    truenas = {
      source  = "truenas/truenas"
      version = "~> 1.5"
    }
  }
}

provider "trueform" {
  host       = "nas.example.com"
  api_key    = var.truenas_api_key
  verify_ssl = true
}

provider "truenas" {
  endpoint = "wss://nas.example.com/api/current"
  api_key  = var.truenas_api_key
  # username = "admin"   # TrueNAS 26+: enables SCRAM-SHA-512 so the key never crosses the wire
}
```

| trueform | truenas | Notes |
|---|---|---|
| `host` / `TRUENAS_HOST` | `endpoint` / `TRUENAS_ENDPOINT` | A full URL: `wss://<host>/api/current`. |
| `api_key` / `TRUENAS_API_KEY` | `api_key` / `TRUENAS_API_KEY` | Unchanged. The same key works for both providers. |
| `verify_ssl = false` / `TRUENAS_VERIFY_SSL` | `insecure = true` | **The meaning is inverted.** There is no environment variable. Alternatively, set `ca_cert = "/path/to/ca.pem"` to trust a self-signed CA properly. |
| — | `username` + `password` | Password auth, an alternative to `api_key`. |

Run `terraform init` to download the official provider.

## Step 2: Generate `removed` and `import` blocks

This prints one `removed` block per trueform resource and one `import` block per instance, using the IDs already in your state (it handles `count`, `for_each` and modules). Save it as `migrate.tf`:

```sh
terraform show -json | jq -r '
def newtype:
  {"trueform_share_smb": "truenas_smb_share",
   "trueform_share_nfs": "truenas_nfs_share",
   "trueform_service_docker": "truenas_docker_config"}[.] // sub("^trueform_"; "truenas_");
[.. | objects | select(.mode? == "managed" and ((.type? // "") | startswith("trueform_")))] as $rs
| ($rs | map(.address | gsub("\\[[^\\]]*\\]"; "")) | unique[]
   | "removed {\n  from = \(.)\n  lifecycle {\n    destroy = false\n  }\n}\n"),
  ($rs[] | . as $r
   | "import {\n  to = \($r.address | sub("trueform_[a-z_]+\\."; ($r.type | newtype) + "."))\n  id = \"\(if $r.type == "trueform_service_docker" then "docker_config" else ($r.values.id | tostring) end)\"\n}\n")
' > migrate.tf
```

Every trueform `id` is already the right import ID for the official provider:

| Resource | Import ID | Example |
|---|---|---|
| pool | numeric ID or pool name | `"1"` or `"tank"` |
| dataset | full path | `"tank/media"` |
| snapshot | `dataset@name` | `"tank/media@pre-upgrade"` |
| smb_share, nfs_share, user, vm, vm_device, cronjob, certificate, static_route, all iSCSI resources | numeric ID (as a string) | `"3"` |
| user | the numeric **database ID**, *not* the UID or username | `"71"` |
| app | app name | `"plex"` |
| docker_config | any string (singleton) | `"docker_config"` |

## Step 3: Rewrite each resource

Rename each `trueform_*` block to its `truenas_*` equivalent, **in place under the same name**, and adjust attributes using the per-resource notes below. Three rules cover most of the risk:

1. **Leave out attributes you don't manage.** Most official attributes are *Optional + Computed* and keep their imported value when you omit them. Omitting is always safe; setting an attribute means its value must match the NAS exactly.
2. **Never set an attribute that can't be read back and forces replacement.** These are null after import, so any value in config forces a replace. Each one is called out below, with an `ignore_changes` workaround where the attribute is required.
3. **Set explicitly the attributes that the official SMB, user and iSCSI resources always send.** On these resources, an unset attribute imports cleanly and plans *No changes*, but as soon as something else on the resource changes it goes *(known after apply)* and is sent as `false` / `""` / `[]`. In testing, changing only an SMB share's `comment` also turned `browsable` off, and changing only a user's `email` removed the user from all its groups. The per-resource notes list which attributes to set. <!-- VERIFY: iscsi_target groups (alias was not affected) -->

References between resources keep working, because the block names stay the same: `truenas_iscsi_target.storage.id` replaces `trueform_iscsi_target.storage.id`.

### Type names

| trueform | truenas |
|---|---|
| `trueform_share_smb` | `truenas_smb_share` |
| `trueform_share_nfs` | `truenas_nfs_share` |
| `trueform_service_docker` | `truenas_docker_config` |
| every other `trueform_X` | `truenas_X` |

## Step 4: Plan, check, apply

```sh
terraform plan
```

Do not apply until the summary reads **`N to import, 0 to add, 0 to destroy`**. The following are expected and fine on the first apply:

- `truenas_dataset.compression` and similar enum attributes switching case (e.g. `LZ4` → `lz4`): an in-place no-op.
- `truenas_app.values` showing as changed: see [Apps](#app). This one *does* call the API.

Anything showing `must be replaced` (`-/+`) is a mistake in config: fix it, don't apply. Then:

```sh
terraform apply
terraform plan     # should now report "No changes"
```

## Step 5: Clean up

Delete `migrate.tf`, the `trueform` entry in `required_providers`, and the `provider "trueform"` block. Run `terraform init` and `terraform plan` once more: it should still say *No changes*. If `~/.terraformrc` has a `dev_overrides` entry for trueform, remove it too.

## Per-resource notes

Only differences are listed. Attributes not mentioned carry over unchanged.

### pool

| trueform | truenas |
|---|---|
| `topology = [{ type = "data", disks = [...] }]` | `topology = { data = [{ type = "MIRROR", disks = [...] }], log = [...], cache = [...], spare = [...] }` |
| `checksum = "on"` | `checksum = "ON"` (case-sensitive, uppercase) |
| `encryption`, `encryption_options` | none: encrypt at the dataset level |
| — | `autotrim`, `guid` |

In trueform, `type` was the vdev *category*, and the layout was inferred from the disk count: 1 disk → STRIPE, 2 → MIRROR, 3+ → RAIDZ1. In the official provider `type` is the layout itself (`STRIPE`, `MIRROR`, `RAIDZ1`–`RAIDZ3`). Write the layout your pool actually has. A wrong topology is refused at plan time; it never plans a destroy. Add `lifecycle { prevent_destroy = true }` while you're here.

If you declared spares with trueform, check the pool: they were probably never attached.

```hcl
resource "truenas_pool" "tank" {
  name     = "tank"
  topology = { data = [{ type = "MIRROR", disks = ["sdb", "sdc"] }] }

  lifecycle {
    prevent_destroy = true
  }
}
```

### dataset

| trueform | truenas |
|---|---|
| `pool = "tank"` + `name = "media"` | `name = "tank/media"` (full path; `pool` is read-only) |
| `deduplication` | `dedup` |
| `aclmode = "passthrough"` etc. | uppercase only: `"PASSTHROUGH"`, `"INHERIT"`, … (also `atime`, `readonly`, `snapdir`, `checksum`) |
| `quota_warning`, `quota_critical`, `casesensitivity`, `managed_by` | none |
| — | `exec`, `sync`, `xattr`, `special_small_block_size`, native encryption |

!> **Remove `type`, `acltype` and `share_type`** from dataset blocks. All three force replacement, and a replace deletes the dataset *recursively*. `share_type` is never read back, and `type`/`acltype` are read back in lowercase. If you must keep `type` or `acltype`, copy the exact lowercase value from `terraform state show` (for example `"filesystem"`, `"inherit"`, `"posix"`).

Nested datasets become a plain full path: `name = "tank/media/movies"`. Use `truenas_dataset.media.mountpoint` rather than hard-coding `/mnt/...` in share paths.

```hcl
resource "truenas_dataset" "media" {
  name        = "tank/media"
  compression = "lz4"
  quota       = 1099511627776
  comments    = "Media storage"
}
```

### snapshot

!> **Remove `recursive`**, even `recursive = false`. It is never read back and it forces replacement, and replacing a snapshot destroys that point-in-time data.

`vmware_sync`, `properties`, `holds`, `referenced_bytes`, `used_bytes` and `creation_time` have no equivalent. For scheduled snapshots, look at the new `truenas_periodic_snapshot_task`.

```hcl
resource "truenas_snapshot" "pre_upgrade" {
  dataset = "tank/media"
  name    = "pre-upgrade"
}
```

### smb_share (was `share_smb`)

| trueform | truenas |
|---|---|
| `path` (in-place) | `path`: now **forces replacement** |
| `name` (forced replacement) | `name`: now in place |
| `audit_logging = true` | `audit = { enable = true }` |
| `path_suffix`, `shadowcopy`, `fsrvp`, `auxsmbconf` | `options = { ... }` (legacy shares only) |
| `hostsallow`, `hostsdeny` | `options = { hostsallow = [...], hostsdeny = [...] }` |
| `home`, `timemachine`, `recyclebin`, `guestok`, `acl`, `durablehandle`, `streams` | only honoured when `purpose = "LEGACY_SHARE"` |

**Always set `purpose`.** trueform created `DEFAULT_SHARE` shares. If `purpose` is unset, the import plans cleanly, but the next update converts the share to `LEGACY_SHARE`. The conversion happens on the NAS even though Terraform then reports *Provider produced inconsistent result after apply*. Also set `enabled`, `browsable`, `ro`, `abe` and `comment` explicitly ([rule 3](#step-3-rewrite-each-resource)). Check `terraform state show` after import to see which `purpose` and `options` your share actually has.

```hcl
resource "truenas_smb_share" "media" {
  name      = "media"
  path      = truenas_dataset.media.mountpoint
  purpose   = "DEFAULT_SHARE"
  comment   = "Media share"
  enabled   = true
  browsable = true
  ro        = false
  abe       = false
}
```

### nfs_share (was `share_nfs`)

This is the easiest migration. `path` now forces replacement, `aliases` is gone, and `id` is a string rather than a number. Everything else carries over.

### user

| trueform | truenas |
|---|---|
| `password` (stored in state) | `password`: **write-only**, never stored in state; requires Terraform ≥ 1.11 |
| `sudo = true` | `sudo_commands = ["ALL"]` <!-- VERIFY --> |
| `sudo_nopasswd = true` | `sudo_commands_nopasswd = ["ALL"]` <!-- VERIFY --> |
| `group_create` | create-only, never read back: **remove** after import |
| `home_mode` | write-only, never read back: keeping it sends it to TrueNAS again on every update of the user. Remove it unless you want that |
| `home_create` | none |
| — | `ssh_password_enabled`, `webshare` (26+), `immutable`, `local` |

`uid` forces replacement: omit it, or match it exactly. trueform's defaults (`home = "/var/empty"`, `shell = "/usr/bin/zsh"`, `smb = true`) no longer apply, so write the values you rely on, along with `full_name`, `email`, `locked`, `password_disabled`, `sshpubkey` and `groups` ([rule 3](#step-3-rewrite-each-resource)).

!> **Copy `groups` from `terraform state show` after import**, even if you never set it in trueform. TrueNAS adds every new user to a built-in group (it was `groups = [91]` on a test system), and the first update of a user with `groups` unset removes the user from it. Leaving `full_name` unset makes that update fail outright (`full_name: String should have at least 1 character`).

### vm

`name` now renames in place, and `ensure_display_device` and `uuid` have no equivalent. Keep `memory` at **the same number you had**: the official docs describe it in bytes, but both providers pass it to TrueNAS unchanged. <!-- VERIFY: unit of vm.memory at the API --> `cpu_mode` values use hyphens (`HOST-PASSTHROUGH`).

!> **Don't set `running`** unless you want Terraform to manage power state. `running = true` starts a stopped VM, and `running = false` shuts a running one down. Also omit `bootloader_ovmf` and `enable_secure_boot`, which force replacement.

### vm_device

The per-type flat attributes become one JSON `attributes` string, which must include `dtype`:

| trueform | inside `attributes` |
|---|---|
| `dtype` | `dtype` |
| `disk_path`, `disk_type` | `path`, `type` |
| `disk_sector_size` | `logical_sectorsize` and `physical_sectorsize` |
| `nic_type`, `nic_mac`, `nic_attach` | `type`, `mac`, `nic_attach` |
| `cdrom_path` | `path` |
| `display_type`, `display_port`, `display_bind`, `display_web`, `display_resolution` | `type`, `port`, `bind`, `web`, `resolution` |
| `display_password` | `attributes_secrets_wo = jsonencode({ password = ... })` + `attributes_secrets_wo_version` |
| `pci_device` | `pptdev` |
| `usb_device` | `device` |
| `raw_path`, `raw_size` | `path`, `size` |

`vm` and `order` stay top-level, and `vm` now forces replacement. Every key you write must equal the server's value, number or string type included. `attributes` is sensitive, so a mismatch shows only as a masked diff. Run `terraform state show` after import to copy the exact values.

```hcl
resource "truenas_vm_device" "disk" {
  vm         = truenas_vm.ubuntu.id
  order      = 1001
  attributes = jsonencode({ dtype = "DISK", path = "/dev/zvol/tank/vms/ubuntu-boot", type = "VIRTIO" })
}
```

<a id="app"></a>
### app

!> **`catalog_app` forces replacement after import**: it is never read back. A replace deletes and redeploys the app. Remove the attribute (simplest; the plan is then clean), or keep it and add `ignore_changes = [catalog_app]`.

Pin `version` to the version currently installed (changing it runs an upgrade), and leave `running` unset. `metadata` is gone; `human_version` and `upgrade_available` are new.

`values` is only compared against the server once state holds a value, and after import it holds none, so the first apply sends it once via `app.update`, which may recreate the app's containers. <!-- VERIFY: app.update with unchanged values restarts containers --> If that matters, add `values` to `ignore_changes` for the migration apply and remove it afterwards.

```hcl
resource "truenas_app" "nginx" {
  name    = "nginx"
  train   = "stable"
  version = "1.4.3"
  values  = jsonencode({ image = { repository = "nginx", tag = "latest" } })
  # catalog_app omitted on purpose: see the warning above
}
```

### docker_config (was `service_docker`)

`pool` and `enable_image_updates` carry over, `status` is gone, and `address_pools`, `cidr_v6` and `registry_mirrors` are new. On TrueNAS 26, remove `nvidia`: setting it there is an error.

The official resource is gentler than trueform's. It never unsets the pool on update, and destroying it doesn't touch the NAS. The `removed` block still matters, though: without it, trueform's destroy *would* unset the Apps pool.

### cronjob

The attributes and `schedule = { ... }` object syntax are the same. When `schedule` is given, all five keys (`minute`, `hour`, `dom`, `month`, `dow`) are now required, because trueform's defaults are gone. `stdout`/`stderr` mean "don't email this output", as they always did on TrueNAS.

### certificate

!> **`create_type` is required, forces replacement, and is null after import.** You *must* add `lifecycle { ignore_changes = [create_type] }`, or the certificate is replaced. Never let that happen to the certificate serving the web UI.

| trueform | truenas |
|---|---|
| `type` | `create_type` (`CERTIFICATE_CREATE_IMPORTED`, `_CSR`, `_IMPORTED_CSR`, `_ACME`; internal CA-signed is not supported) |
| `common_name` | `common` |
| `signedby`, `lifetime`, `cert_chain`, `not_before`, `not_after` | none |
| `san = ["DNS:host"]` | `san = ["host"]` |

`certificate`, `privatekey`, `csr`, `key_type`, `key_length`, `digest_algorithm` and the subject fields (`country`, `state`, `city`, `organization`, …) all force replacement. Omit them after import unless they match the NAS byte for byte.

```hcl
resource "truenas_certificate" "web" {
  name        = "web"
  create_type = "CERTIFICATE_CREATE_IMPORTED"

  lifecycle {
    ignore_changes = [create_type]
  }
}
```

### static_route

No changes beyond the type name.

### iSCSI (portal, initiator, target, extent, targetextent)

Only the type prefix changes, and the import IDs are the same numbers. The differences:

- **portal:** `discovery_authmethod` and `discovery_authgroup` are gone (discovery auth lives on `truenas_iscsi_auth`). `listen[].port` is read-only, and the port is set globally with `truenas_iscsi_global.listen_port`.
- **initiator:** `auth_network` moved to `auth_networks` on `truenas_iscsi_target`. Always set `initiators` explicitly: an empty list means *allow all*.
- **target:** `name` now changes in place (the IQN still changes, so initiators reconnect). In `groups`, set **all four** of `portal`, `initiator`, `auth` (0 = none) and `authmethod` on every entry.
- **extent:** `name` still forces replacement and must match exactly. If you set `avail_threshold` on the NAS, carry it over: it is always sent.
- **targetextent:** `target` and `extent` now force replacement. Keep `lunid` explicit, because changing a LUN disrupts initiators.

```hcl
resource "truenas_iscsi_target" "storage" {
  name  = "storage"
  alias = "Storage Target"
  mode  = "ISCSI"
  groups = [{
    portal     = truenas_iscsi_portal.default.id
    initiator  = truenas_iscsi_initiator.trusted.id
    auth       = 0
    authmethod = "NONE"
  }]
}
```

## Data sources

Data sources hold no real state, so just rename them. The lookup keys are stricter:

| trueform | truenas | Change |
|---|---|---|
| `data "trueform_pool"` (`id` or `name`) | `data "truenas_pool"` | `name` required; `fragmentation` gone |
| `data "trueform_dataset"` (`id = "tank/media"`) | `data "truenas_dataset"` | look up by `name = "tank/media"`; `deduplication` → `dedup`; `used`, `available`, `key_loaded` gone |
| `data "trueform_user"` (`id`, `uid` or `username`) | `data "truenas_user"` | `username` required; `sudo`/`sudo_nopasswd` → `sudo_commands*` |
| `data "trueform_vm"` (`id` or `name`) | `data "truenas_vm"` | `name` required; `uuid` gone |

## If something goes wrong

Before apply, nothing has changed: fix the config and plan again. After a bad apply, restore the state with `terraform state push pre-migration.tfstate` (add `-force` if Terraform objects to the serial), revert your configuration, and re-plan with trueform. If an object on the NAS was actually replaced, restore it from your snapshot or TrueNAS config backup.

## After migrating

Things you could not express with trueform and can now manage: `truenas_zvol` (VM disks and iSCSI backing), `truenas_group`, `truenas_periodic_snapshot_task`, `truenas_replication_task`, `truenas_service` (SMB/NFS/SSH on/off), `truenas_filesystem_acl`, and per-service config such as `truenas_smb_config` and `truenas_nfs_config`. See the [official provider docs](https://registry.terraform.io/providers/truenas/truenas/latest/docs).
