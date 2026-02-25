# Contributing

## Test VM Setup (Proxmox)

The integration tests in `test-resources/` require a TrueNAS SCALE VM with disposable disks. This guide covers setting up the test VM on Proxmox VE, taking a clean snapshot, and the snapshot-revert workflow for repeatable test runs.

### 1. Download TrueNAS SCALE ISO

Download TrueNAS SCALE 25.04 (or newer) from https://www.truenas.com/download-truenas-scale/ and upload it to a Proxmox storage backend that supports ISO images (e.g., `local` or an NFS ISO library).

### 2. Create the VM

Create a new VM in the Proxmox UI with the following specs:

| Setting | Value |
|---------|-------|
| CPU | 2 cores |
| RAM | 8 GB |
| Boot disk | 32 GB SCSI on any storage backend |
| Test disks | 4x 512 MB SCSI disks on any storage backend |
| Network | 1x virtio NIC on a bridged network with DHCP |
| OS | Attach the TrueNAS SCALE ISO |

The 4 test disks are used by the integration tests to create and destroy ZFS pools. They must be small (512 MB is sufficient) and separate from the boot disk.

**Disk naming:** Proxmox SCSI disks appear as `sd*` in the guest. The boot disk is typically `sda`, so the 4 test disks will be `sdb`, `sdc`, `sdd`, `sde`. Verify after install using the `disk.query` API (see step 5).

### 3. Install TrueNAS

Boot the VM from the ISO and follow the TrueNAS installer:

1. Select the 32 GB boot disk as the install target
2. Set the admin password
3. Reboot (remove the ISO from the CD drive first)

### 4. Initial Configuration

After the VM boots into TrueNAS:

1. Complete the setup wizard in the web UI
2. Navigate to **Credentials > API Keys > Add**
3. Create an API key and save it — you'll need it for `terraform.tfvars`

Store the VM's IP and API key in `test-resources/creds` (gitignored):

```
IP: <vm-ip>
API_KEY: <api-key>
```

### 5. Discover Disk Names

Query the TrueNAS API to confirm the disk identifiers:

```bash
curl -k -X POST "https://<HOST>/api/current" \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"msg":"method","method":"disk.query","params":[[],{"select":["name","size","type"]}]}'
```

You should see `sda` (boot) and `sdb`, `sdc`, `sdd`, `sde` (test disks). Update `pool_disks` in the `terraform.tfvars` files accordingly.

### 6. Take a Clean Snapshot

In the Proxmox UI, take a VM snapshot (e.g., named `clean-install`). This captures the fresh TrueNAS state before any test resources are created.

### Snapshot-Revert Workflow

Before each full test cycle (`create → import → modify → destroy`), revert the VM to the `clean-install` snapshot. This guarantees:

- No leftover pools, datasets, shares, or users from previous runs
- The API key remains valid (it was created before the snapshot)
- Disk identifiers stay consistent

To revert via the Proxmox API or CLI:

```bash
# CLI (on the Proxmox host)
qm snapshot <vmid> rollback clean-install

# API
curl -X POST "https://<proxmox-host>:8006/api2/json/nodes/<node>/qemu/<vmid>/snapshot/clean-install/rollback" \
  -H "Authorization: PVEAPIToken=<token>"
```

Or simply use the Proxmox UI: select the VM → Snapshots → `clean-install` → Rollback.

After reverting, wait for TrueNAS to boot (~30–60 seconds), then run the test cycle as described in `test-resources/README.md`.
