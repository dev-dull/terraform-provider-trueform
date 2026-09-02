// Package proxmox is a minimal Proxmox VE API-token client for the TrueNAS e2e
// test harness. It speaks only the handful of endpoints the harness needs
// (snapshot list, VM status, and later rollback/snapshot).
//
// Homelab test tooling: this package is not part of the shipped provider and is
// never compiled into a release binary.
//
// Safety: every call is guarded by a vmid allowlist built from the harness
// config. The client refuses to touch any VM not explicitly configured, so a
// destructive rollback can never land on the wrong guest.
package proxmox

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client talks to one Proxmox node's API with a scoped API token.
type Client struct {
	apiURL  string // e.g. https://192.168.1.90:8006
	node    string // e.g. borg-queen
	tokenID string // e.g. trueform-test@pve!tester
	secret  string
	allowed map[int]bool
	http    *http.Client
}

// New builds a client. verifyTLS=false skips certificate verification, which the
// homelab PVE requires (self-signed cert). allowedVMIDs is the safety allowlist.
func New(apiURL, node, tokenID, secret string, verifyTLS bool, allowedVMIDs []int) *Client {
	allowed := make(map[int]bool, len(allowedVMIDs))
	for _, id := range allowedVMIDs {
		allowed[id] = true
	}
	return &Client{
		apiURL:  apiURL,
		node:    node,
		tokenID: tokenID,
		secret:  secret,
		allowed: allowed,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifyTLS}, //nolint:gosec // homelab PVE uses a self-signed cert
			},
		},
	}
}

// Snapshot is one entry from the qemu snapshot list. Proxmox always includes a
// synthetic entry named "current" for the live HEAD; it has no SnapTime/VMState.
type Snapshot struct {
	Name        string `json:"name"`
	Parent      string `json:"parent"`
	SnapTime    int64  `json:"snaptime"`
	Description string `json:"description"`
	VMState     int    `json:"vmstate"` // 1 if the snapshot captured RAM; 0 = disk-only
}

// IsCurrent reports whether this is the synthetic live-HEAD entry, not a real
// restore point.
func (s Snapshot) IsCurrent() bool { return s.Name == "current" }

// Status is the subset of a VM's current status the harness uses.
type Status struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	QMPStatus string `json:"qmpstatus"`
}

func (c *Client) guard(vmid int) error {
	if !c.allowed[vmid] {
		return fmt.Errorf("vmid %d is not in the configured allowlist; refusing to act on it", vmid)
	}
	return nil
}

func (c *Client) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.apiURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+c.secret)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxmox GET %s: %s", path, resp.Status)
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return json.Unmarshal(env.Data, out)
}

// ListSnapshots returns the VM's snapshots, including the synthetic "current".
func (c *Client) ListSnapshots(vmid int) ([]Snapshot, error) {
	if err := c.guard(vmid); err != nil {
		return nil, err
	}
	var snaps []Snapshot
	if err := c.get(fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/snapshot", c.node, vmid), &snaps); err != nil {
		return nil, err
	}
	return snaps, nil
}

// Status returns the VM's current run status.
func (c *Client) Status(vmid int) (*Status, error) {
	if err := c.guard(vmid); err != nil {
		return nil, err
	}
	var st Status
	if err := c.get(fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/current", c.node, vmid), &st); err != nil {
		return nil, err
	}
	return &st, nil
}
