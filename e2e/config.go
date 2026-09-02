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
	Targets map[string]TargetConfig `json:"targets"`
}

type ProxmoxConfig struct {
	APIURL    string `json:"api_url"`
	Node      string `json:"node"`
	TokenID   string `json:"token_id"`
	VerifyTLS bool   `json:"verify_tls"`
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
