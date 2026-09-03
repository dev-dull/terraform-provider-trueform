// Command e2e is the TrueNAS integration-test harness: it drives Proxmox
// snapshots and (in later phases) the terraform test cycle against TrueNAS 25
// and 26, so provider changes can be validated end-to-end before release.
//
// Homelab test tooling — not part of the shipped provider, never in a release.
//
// The Proxmox token secret is read from the PROXMOX_TOKEN_SECRET environment
// variable, never from a file. Resolve it from 1Password at the call site:
//
//	PROXMOX_TOKEN_SECRET="$(op read 'op://Homelab/Proxmox Trueform Test/password')" \
//	    go run ./e2e status
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/trueform/terraform-provider-trueform/e2e/proxmox"
	"github.com/trueform/terraform-provider-trueform/e2e/runner"
	"github.com/trueform/terraform-provider-trueform/e2e/truenas"
)

// bootTimeout bounds how long we wait for TrueNAS to answer after a rollback+start.
const bootTimeout = 6 * time.Minute

// updateTimeout bounds an update.run: download (~GBs) + install + reboot.
const updateTimeout = 40 * time.Minute

const defaultConfigPath = "e2e/config.json"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "status":
		if err := runStatus(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "reset":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: e2e reset <target>")
			os.Exit(2)
		}
		if err := runReset(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "test":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: e2e test <target>")
			os.Exit(2)
		}
		if err := runTest(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "check-updates":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: e2e check-updates <target>")
			os.Exit(2)
		}
		if err := runCheckUpdates(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "refresh":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: e2e refresh <target>")
			os.Exit(2)
		}
		if err := runRefresh(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: e2e <command>")
	fmt.Fprintln(os.Stderr, "  status          read-only: Proxmox connectivity + per-target snapshot/VM state")
	fmt.Fprintln(os.Stderr, "  reset <target>   DESTRUCTIVE: roll <target> back to its golden, boot, verify health")
	fmt.Fprintln(os.Stderr, "  test <target>    DESTRUCTIVE: reset, then run the full create->import->modify->destroy cycle")
	fmt.Fprintln(os.Stderr, "  check-updates <target>  DESTRUCTIVE: reset, then report whether a TrueNAS update is available")
	fmt.Fprintln(os.Stderr, "  refresh <target>  DESTRUCTIVE: reset, apply any TrueNAS update, snapshot a new golden, repoint config")
}

// goldenName derives a Proxmox snapshot name from a version, matching the existing
// convention (e.g. "25.10.7" -> "Updated_to_25_10_7", "26.0.0-BETA.3" ->
// "Updated_to_26_0_0_BETA_3").
func goldenName(version string) string {
	return "Updated_to_" + strings.NewReplacer(".", "_", "-", "_").Replace(version)
}

func target(cfg *Config, name string) (TargetConfig, error) {
	t, ok := cfg.Targets[name]
	if !ok {
		names := make([]string, 0, len(cfg.Targets))
		for n := range cfg.Targets {
			names = append(names, n)
		}
		sort.Strings(names)
		return TargetConfig{}, fmt.Errorf("unknown target %q (configured: %v)", name, names)
	}
	return t, nil
}

func truenasAPIKey() (string, error) {
	key := os.Getenv("TRUENAS_API_KEY")
	if key == "" {
		return "", fmt.Errorf("TRUENAS_API_KEY is empty; export the TrueNAS API key (e.g. from test-resources/creds or 1Password)")
	}
	return key, nil
}

// resetTarget rolls a target back to its golden snapshot, boots the VM, and
// verifies the guest is healthy and the expected major version. DESTRUCTIVE: it
// discards the VM's current live state.
func resetTarget(ctx context.Context, cfg *Config, pve *proxmox.Client, t TargetConfig, name, apiKey string) error {
	fmt.Printf("reset %s: rolling vmid %d back to golden %q (DESTRUCTIVE: discards live state)\n", name, t.VMID, t.Golden)
	start := time.Now()
	if err := pve.Rollback(ctx, t.VMID, t.Golden); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	fmt.Printf("  rollback complete (%s); waiting for TrueNAS to boot...\n", time.Since(start).Round(time.Second))

	tn, err := truenas.WaitForReady(ctx, cfg.Truenas.Host, apiKey, bootTimeout)
	if err != nil {
		return err
	}
	defer tn.Close()

	ver, _ := tn.Version(ctx)
	fmt.Printf("  ready (%s): %s (major %d)\n", time.Since(start).Round(time.Second), ver, tn.MajorVersion())

	if err := tn.HealthCheck(ctx, t.ExpectedMajor); err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	pools, _ := tn.PoolCount(ctx)
	fmt.Printf("  healthy: major %d as expected, %d user pool(s) online\n", t.ExpectedMajor, pools)
	return nil
}

// setup loads config, resolves the target and creds, and builds the Proxmox
// client — the common preamble for reset and test.
func setup(name string) (*Config, TargetConfig, *proxmox.Client, string, error) {
	cfg, err := LoadConfig(configPath())
	if err != nil {
		return nil, TargetConfig{}, nil, "", err
	}
	t, err := target(cfg, name)
	if err != nil {
		return nil, TargetConfig{}, nil, "", err
	}
	pve, err := newProxmox(cfg)
	if err != nil {
		return nil, TargetConfig{}, nil, "", err
	}
	apiKey, err := truenasAPIKey()
	if err != nil {
		return nil, TargetConfig{}, nil, "", err
	}
	return cfg, t, pve, apiKey, nil
}

func runReset(name string) error {
	cfg, t, pve, apiKey, err := setup(name)
	if err != nil {
		return err
	}
	return resetTarget(context.Background(), cfg, pve, t, name, apiKey)
}

// runTest resets the target to a clean golden, then runs the full terraform
// create->import->modify->destroy cycle against it. DESTRUCTIVE.
func runTest(name string) error {
	cfg, t, pve, apiKey, err := setup(name)
	if err != nil {
		return err
	}
	if err := cfg.Test.Validate(); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if err := resetTarget(ctx, cfg, pve, t, name, apiKey); err != nil {
		return err
	}

	fmt.Printf("test %s: create -> import -> modify -> destroy\n", name)
	rn, err := runner.New(root, runner.Params{
		TruenasHost:        cfg.Truenas.Host,
		TruenasAPIKey:      apiKey,
		PoolName:           cfg.Test.PoolName,
		PoolDisks:          cfg.Test.PoolDisks,
		TestPrefix:         cfg.Test.TestPrefix,
		BasePath:           cfg.Test.BasePath,
		NFSAllowedNetwork:  cfg.Test.NFSAllowedNetwork,
		StaticRouteGateway: cfg.Test.StaticRouteGateway,
		AppVersion:         cfg.Test.AppVersion,
	})
	if err != nil {
		return err
	}
	defer func() { _ = rn.Close() }()

	if err := rn.RunCycle(ctx); err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}
	fmt.Printf("test %s: PASS\n", name)
	return nil
}

// runCheckUpdates resets the target to its golden and reports whether a TrueNAS
// update is available on that line. DESTRUCTIVE (the reset discards live state).
func runCheckUpdates(name string) error {
	cfg, t, pve, apiKey, err := setup(name)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := resetTarget(ctx, cfg, pve, t, name, apiKey); err != nil {
		return err
	}
	tn, err := truenas.WaitForReady(ctx, cfg.Truenas.Host, apiKey, bootTimeout)
	if err != nil {
		return err
	}
	defer tn.Close()
	info, err := tn.CheckUpdate(ctx)
	if err != nil {
		return fmt.Errorf("update.status: %w", err)
	}
	if info.Available {
		fmt.Printf("check-updates %s: UPDATE AVAILABLE -> %s (train %s)\n", name, info.NewVersion, info.CurrentTrain)
	} else {
		fmt.Printf("check-updates %s: up to date (train %s)\n", name, info.CurrentTrain)
	}
	return nil
}

// runRefresh resets the target to its golden, applies any available TrueNAS update
// (which reboots), snapshots a new golden, and repoints the config to it.
// DESTRUCTIVE, and slow (an update is a multi-GB download + install + reboot).
func runRefresh(name string) error {
	cfg, t, pve, apiKey, err := setup(name)
	if err != nil {
		return err
	}
	ctx := context.Background()

	if err := resetTarget(ctx, cfg, pve, t, name, apiKey); err != nil {
		return err
	}

	tn, err := truenas.WaitForReady(ctx, cfg.Truenas.Host, apiKey, bootTimeout)
	if err != nil {
		return err
	}
	info, err := tn.CheckUpdate(ctx)
	if err != nil {
		_ = tn.Close()
		return fmt.Errorf("update.status: %w", err)
	}
	if !info.Available {
		_ = tn.Close()
		fmt.Printf("refresh %s: already up to date (train %s); golden unchanged\n", name, info.CurrentTrain)
		return nil
	}

	fmt.Printf("refresh %s: applying update -> %s (downloads + installs + reboots; up to %s)...\n", name, info.NewVersion, updateTimeout)
	if err := tn.ApplyUpdate(ctx); err != nil {
		_ = tn.Close()
		return fmt.Errorf("update.run: %w", err)
	}
	_ = tn.Close()

	start := time.Now()
	tn2, err := truenas.WaitForVersion(ctx, cfg.Truenas.Host, apiKey, info.NewVersion, updateTimeout)
	if err != nil {
		return err
	}
	defer tn2.Close()
	ver, _ := tn2.Version(ctx)
	fmt.Printf("  updated (%s): %s\n", time.Since(start).Round(time.Second), ver)
	if err := tn2.HealthCheck(ctx, t.ExpectedMajor); err != nil {
		return fmt.Errorf("post-update health: %w", err)
	}

	snap := goldenName(info.NewVersion)
	fmt.Printf("  snapshotting new golden %q...\n", snap)
	if err := pve.CreateSnapshot(ctx, t.VMID, snap, "auto: updated to "+info.NewVersion); err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	if err := cfg.SetGolden(configPath(), name, snap); err != nil {
		return fmt.Errorf("repoint golden in config: %w", err)
	}
	fmt.Printf("refresh %s: DONE — new golden %q, config repointed\n", name, snap)
	return nil
}

func configPath() string {
	if p := os.Getenv("E2E_CONFIG"); p != "" {
		return p
	}
	return defaultConfigPath
}

func newProxmox(cfg *Config) (*proxmox.Client, error) {
	secret := os.Getenv("PROXMOX_TOKEN_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("PROXMOX_TOKEN_SECRET is empty; resolve it from 1Password (op read) and export it — is the 1Password app unlocked?")
	}
	return proxmox.New(cfg.Proxmox.APIURL, cfg.Proxmox.Node, cfg.Proxmox.TokenID, secret, cfg.Proxmox.VerifyTLS, cfg.AllowedVMIDs()), nil
}

func runStatus() error {
	cfg, err := LoadConfig(configPath())
	if err != nil {
		return err
	}
	pve, err := newProxmox(cfg)
	if err != nil {
		return err
	}

	// deterministic target order
	names := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("Proxmox %s  node=%s  token=%s\n\n", cfg.Proxmox.APIURL, cfg.Proxmox.Node, cfg.Proxmox.TokenID)

	for _, name := range names {
		t := cfg.Targets[name]
		fmt.Printf("== %s  (vmid %d, expected TrueNAS %d) ==\n", name, t.VMID, t.ExpectedMajor)

		st, err := pve.Status(t.VMID)
		if err != nil {
			fmt.Printf("  status: ERROR %v\n\n", err)
			continue
		}
		fmt.Printf("  vm: %s  status=%s  qmp=%s\n", st.Name, st.Status, st.QMPStatus)

		snaps, err := pve.ListSnapshots(t.VMID)
		if err != nil {
			fmt.Printf("  snapshots: ERROR %v\n\n", err)
			continue
		}

		goldenFound := false
		for _, s := range snaps {
			if s.Name == t.Golden {
				goldenFound = true
				break
			}
		}
		mark := "MISSING"
		if goldenFound {
			mark = "present"
		}
		fmt.Printf("  golden %q: %s\n", t.Golden, mark)

		// snaptime asc; the synthetic "current" (snaptime 0) sorts to the end.
		key := func(s proxmox.Snapshot) int64 {
			if s.SnapTime == 0 {
				return 1 << 62
			}
			return s.SnapTime
		}
		sort.Slice(snaps, func(i, j int) bool { return key(snaps[i]) < key(snaps[j]) })

		fmt.Println("  snapshot chain:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, s := range snaps {
			when := "live HEAD"
			if s.SnapTime > 0 {
				when = time.Unix(s.SnapTime, 0).UTC().Format("2006-01-02 15:04")
			}
			ram := ""
			if s.VMState == 1 {
				ram = "  +RAM"
			}
			flag := ""
			if s.Name == t.Golden {
				flag = "  <- golden"
			}
			fmt.Fprintf(w, "    %s\t%s%s%s\n", s.Name, when, ram, flag)
		}
		_ = w.Flush()
		fmt.Println()
	}
	return nil
}
