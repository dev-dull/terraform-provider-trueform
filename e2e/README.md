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
