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

`<target>` is a key under `targets` in the config (e.g. `tn25`, `tn26`).

The cycle builds the provider and runs it via `dev_overrides` (`TF_CLI_CONFIG_FILE`), so terraform is never `init`-ed against a registry. Each phase runs in a temp copy of the `test-resources` phase dir with credentials/params injected as `TF_VAR_*` — the tracked configs are never mutated and no secret is written to disk. On failure the temp workdir is preserved and its path printed.

## Golden maintenance & the update–test cycle

The harness tests the provider against a **golden** snapshot per TrueNAS version —
an up-to-date base to run the provider cycle against. TrueNAS updates themselves are
trusted (iX tests them); the goal here is testing the `trueform` provider, not
TrueNAS. To keep testing against the latest TrueNAS, refresh each golden, then run
the provider cycle. Do the two versions sequentially (there is one VM).

### Per version (25 first, then 26)

1. **Revert to the current golden** — `e2e reset tn25` (or the Proxmox UI).
2. **Run TrueNAS updates** and let it reboot. Manual today. For **26** this is a
   BETA/RC/GA step and is a **human** decision until 26.0 GA.
3. **Take a fresh snapshot** of the updated VM — a new, *unique* name (snapshots
   **cannot be renamed**), e.g. `Updated_to_25_10_5`, with a description. This is the
   new golden.
4. **Repoint the golden** — set `targets.tn25.golden` in `config.json` to the new
   snapshot name. Until you do, `e2e test` still uses the *old* golden.
5. **Test the provider** — `e2e test tn25` runs the full create → import → modify →
   destroy cycle against the new golden.

`e2e test` begins by reverting to the golden and booting it (with a quick
version/pools sanity check), so that step also confirms the new golden is usable —
no separate verification needed. A cycle failure tied to a changed TrueNAS API shape
is the signal that the provider needs updating for that version.

Then repeat for **26** (`tn26`); it reverts to the 26 golden independently — the
25-line work does not affect it.

### Snapshot-tree housekeeping

Goldens form a **branching tree** on the one VM — each refresh branches from the
golden you reverted to (the 26 golden is already a child of the 25 golden). Proxmox
**cannot rename a snapshot, and cannot delete one that still has children**, so each
refresh adds a snapshot and old goldens cannot be pruned while they are ancestors of
newer ones. The chain — and its disk usage — grows over time;
periodically re-baseline from a fresh clean-install lineage to keep it bounded.

### Automated vs manual (today)

- **Automated:** revert + boot (`e2e reset`), and the full test cycle (`e2e test`).
- **Manual:** running the TrueNAS update, taking the snapshot, and repointing the
  golden in `config.json`. A future `e2e refresh <target>` would fold these in —
  apply minor-within-train updates, snapshot, and repoint the golden **only on a
  green cycle** — with beta/major updates left to a human.
