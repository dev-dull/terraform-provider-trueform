package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the harness configuration, loaded from a gitignored JSON file. It
// holds no secrets: the Proxmox token secret comes from the environment
// (PROXMOX_TOKEN_SECRET), never from this file.
type Config struct {
	Proxmox ProxmoxConfig           `json:"proxmox"`
	Truenas TruenasConfig           `json:"truenas"`
	Test    TestConfig              `json:"test"`
	Targets map[string]TargetConfig `json:"targets"`
}

// TestConfig holds the non-secret parameters the terraform test cycle needs
// (disk names, networks, etc.). The API key is never here — it comes from the
// TRUENAS_API_KEY environment variable.
type TestConfig struct {
	PoolName           string   `json:"pool_name"`
	PoolDisks          []string `json:"pool_disks"`
	TestPrefix         string   `json:"test_prefix"`
	BasePath           string   `json:"base_path"`
	NFSAllowedNetwork  string   `json:"nfs_allowed_network"`
	StaticRouteGateway string   `json:"static_route_gateway"`
	AppVersion         string   `json:"app_version"`
}

// Validate reports whether the test section has the fields the cycle requires.
// Called only by `e2e test`, so `status`/`reset` work without a test section.
func (t TestConfig) Validate() error {
	missing := []string{}
	if t.PoolName == "" {
		missing = append(missing, "pool_name")
	}
	if len(t.PoolDisks) == 0 {
		missing = append(missing, "pool_disks")
	}
	if t.NFSAllowedNetwork == "" {
		missing = append(missing, "nfs_allowed_network")
	}
	if t.StaticRouteGateway == "" {
		missing = append(missing, "static_route_gateway")
	}
	if t.AppVersion == "" {
		missing = append(missing, "app_version")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config test section is missing: %v", missing)
	}
	return nil
}

type ProxmoxConfig struct {
	APIURL    string `json:"api_url"`
	Node      string `json:"node"`
	TokenID   string `json:"token_id"`
	VerifyTLS bool   `json:"verify_tls"`
}

// TruenasConfig is the guest's connection info. It is the same VM across both
// targets (one IP, one API key that persists across rollbacks). The API key is
// NOT stored here — it comes from the TRUENAS_API_KEY environment variable.
type TruenasConfig struct {
	Host string `json:"host"`
}

type TargetConfig struct {
	VMID          int    `json:"vmid"`
	Golden        string `json:"golden"` // exact Proxmox snapshot name (never derived)
	ExpectedMajor int    `json:"expected_major"`
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Proxmox.APIURL == "" || c.Proxmox.Node == "" || c.Proxmox.TokenID == "" {
		return nil, fmt.Errorf("%s: proxmox.api_url, node, and token_id are required", path)
	}
	if c.Truenas.Host == "" {
		return nil, fmt.Errorf("%s: truenas.host is required", path)
	}
	if len(c.Targets) == 0 {
		return nil, fmt.Errorf("%s: at least one target is required", path)
	}
	return &c, nil
}

// AllowedVMIDs returns the unique vmids across all targets, for the Proxmox
// client's safety allowlist.
func (c *Config) AllowedVMIDs() []int {
	seen := map[int]bool{}
	var ids []int
	for _, t := range c.Targets {
		if !seen[t.VMID] {
			seen[t.VMID] = true
			ids = append(ids, t.VMID)
		}
	}
	return ids
}
