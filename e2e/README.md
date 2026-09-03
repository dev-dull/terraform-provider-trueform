# e2e — TrueNAS integration test harness

Homelab test tooling for the trueform provider. It drives Proxmox VM snapshots
and the terraform test cycle to validate provider changes against TrueNAS 25 and
26 end-to-end.

**Not part of the shipped provider.** Nothing here is compiled into a release
binary (GoReleaser builds only the provider's root package). It lives in this
module for now so it can reuse `internal/client`; it is destined to move to a
private Gitea repo.

## Layout

- `proxmox/` — minimal Proxmox VE API-token client (snapshot list, VM status;
  rollback/snapshot in later phases). Every call is guarded by a vmid allowlist.
- `main.go` / `config.go` — the `e2e` command and its config loader.
- `config.json` — **gitignored**; copy `config.example.json` and fill in real
  values (host, node, token id, vmids, exact golden snapshot names).

Later phases add TrueNAS readiness/health/version (via `internal/client`) and the
terraform runner.

## Secrets

The Proxmox token secret is read from `$PROXMOX_TOKEN_SECRET`, never from a file,
tfvars, or source. Resolve it from 1Password at the call site:

```sh
PROXMOX_TOKEN_SECRET="$(op read 'op://Homelab/Proxmox Trueform Test/password')" \
    go run ./e2e status
```

If the 1Password app is locked, `op` returns an empty string and the harness
errors with a clear message — unlock the app and retry. For unattended/CI use a
1Password service-account token instead of desktop integration.

## Commands

- `e2e status` — read-only: Proxmox connectivity + per-target snapshot/VM state.
- `e2e reset <target>` — **DESTRUCTIVE**: roll the target back to its golden snapshot, boot, and verify health (major version + pools online). Leaves the VM in a known-good state.
- `e2e test <target>` — **DESTRUCTIVE**: `reset`, then run the full create → import → modify → destroy cycle and assert the invariants (create succeeds, the post-import plan is clean, destroy empties the state). Requires `TRUENAS_API_KEY` and the `test` section of `config.json`.
- `e2e check-updates <target>` — **DESTRUCTIVE**: `reset`, then report whether a TrueNAS update is available on that line (via `update.status`).
- `e2e refresh <target>` — **DESTRUCTIVE**: `reset`, apply any available TrueNAS update (`update.run`, which downloads + installs + reboots), health-check the new version, snapshot a new golden (`Updated_to_<version>`), and repoint `targets.<target>.golden` in `config.json`. No-op if already current.

`<target>` is a key under `targets` in the config (e.g. `tn25`, `tn26`).

The cycle builds the provider and runs it via `dev_overrides` (`TF_CLI_CONFIG_FILE`), so terraform is never `init`-ed against a registry. Each phase runs in a temp copy of the `test-resources` phase dir with credentials/params injected as `TF_VAR_*` — the tracked configs are never mutated and no secret is written to disk. On failure the temp workdir is preserved and its path printed.

## Golden maintenance & the update–test cycle

The harness tests the provider against a **golden** snapshot per TrueNAS version —
an up-to-date base to run the provider cycle against. TrueNAS updates themselves are
trusted (iX tests them); the goal here is testing the `trueform` provider, not
TrueNAS. To keep testing against the latest TrueNAS, refresh each golden, then run
the provider cycle. Do the two versions sequentially (there is one VM).

### Per version (25 first, then 26)

For each line it is two commands:

1. **`e2e refresh tn25`** — reverts to the current golden, applies any available
   TrueNAS update (reboots), health-checks the new version, snapshots a new golden
   (`Updated_to_<version>`), and repoints `targets.tn25.golden` in `config.json`. If
   the line is already current it is a no-op and the golden is unchanged.
2. **`e2e test tn25`** — runs the full create → import → modify → destroy cycle
   against the (now current) golden.

`e2e test` begins by reverting to the golden and booting it (with a quick
version/pools sanity check), so that step also confirms the golden is usable — no
separate verification needed. A cycle failure tied to a changed TrueNAS API shape is
the signal that the provider needs updating for that version.

Then repeat for **26** (`e2e refresh tn26` / `e2e test tn26`); it reverts to the 26
golden independently — the 25-line work does not affect it. **26 updates are a
BETA/RC/GA step**: `refresh` will apply whatever the beta train offers, so run it on
26 only when you intend to move the beta golden forward (a human decision until 26.0
GA).

### Snapshot-tree housekeeping

Goldens form a **branching tree** on the one VM — each refresh branches from the
golden you reverted to (the 26 golden is already a child of the 25 golden). Proxmox
**cannot rename a snapshot, and cannot delete one that still has children**, so each
refresh adds a snapshot and old goldens cannot be pruned while they are ancestors of
newer ones. The chain — and its disk usage — grows over time;
periodically re-baseline from a fresh clean-install lineage to keep it bounded.

### What's automated

`refresh` and `test` together automate the whole loop: revert, apply update, health-
check, snapshot the new golden, repoint config, and run the provider cycle. The
remaining human decisions are *when* to run `refresh` on the 26 (beta) line, and
periodic re-baselining of the snapshot tree.

`refresh` blesses a new golden on **TrueNAS health** after the update (it does not
gate on the provider `test`): a `test` failure after an update usually means the
provider needs work for the new version, not that the golden is bad.
