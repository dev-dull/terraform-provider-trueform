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
