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
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/trueform/terraform-provider-trueform/e2e/proxmox"
)

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
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: e2e <command>")
	fmt.Fprintln(os.Stderr, "  status   read-only: Proxmox connectivity + per-target snapshot/VM state")
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
