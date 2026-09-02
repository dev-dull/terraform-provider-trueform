// Package proxmox is a minimal Proxmox VE API-token client for the TrueNAS e2e
// test harness. It speaks only the handful of endpoints the harness needs
// (snapshot list, VM status, rollback).
//
// Homelab test tooling: this package is not part of the shipped provider and is
// never compiled into a release binary.
//
// Safety: every call is guarded by a vmid allowlist built from the harness
// config. The client refuses to touch any VM not explicitly configured, so a
// destructive rollback can never land on the wrong guest.
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func (c *Client) do(method, path string, form url.Values, out interface{}) error {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequest(method, c.apiURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+c.secret)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxmox %s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
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
	if err := c.do(http.MethodGet, fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/snapshot", c.node, vmid), nil, &snaps); err != nil {
		return nil, err
	}
	return snaps, nil
}

// HasSnapshot reports whether a snapshot with the exact name exists on the VM.
func (c *Client) HasSnapshot(vmid int, name string) (bool, error) {
	snaps, err := c.ListSnapshots(vmid)
	if err != nil {
		return false, err
	}
	for _, s := range snaps {
		if s.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Status returns the VM's current run status.
func (c *Client) Status(vmid int) (*Status, error) {
	if err := c.guard(vmid); err != nil {
		return nil, err
	}
	var st Status
	if err := c.do(http.MethodGet, fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/current", c.node, vmid), nil, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// Rollback reverts the VM to a snapshot and starts it. DESTRUCTIVE: it discards
// the VM's live state. It refuses a vmid outside the allowlist or a snapshot
// name that does not exist, then blocks until the rollback task completes
// successfully. Proxmox stops a running VM as part of rollback; start=1 boots it
// afterward (the harness then waits on the guest OS separately).
func (c *Client) Rollback(ctx context.Context, vmid int, snapshot string) error {
	if err := c.guard(vmid); err != nil {
		return err
	}
	ok, err := c.HasSnapshot(vmid, snapshot)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("snapshot %q does not exist on vmid %d; refusing to roll back", snapshot, vmid)
	}

	var upid string
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/snapshot/%s/rollback", c.node, vmid, url.PathEscape(snapshot))
	if err := c.do(http.MethodPost, path, url.Values{"start": {"1"}}, &upid); err != nil {
		return fmt.Errorf("start rollback: %w", err)
	}
	return c.waitTask(ctx, upid)
}

// waitTask polls a Proxmox task (UPID) until it stops, returning an error unless
// it exited "OK".
func (c *Client) waitTask(ctx context.Context, upid string) error {
	path := fmt.Sprintf("/api2/json/nodes/%s/tasks/%s/status", c.node, url.PathEscape(upid))
	deadline := time.Now().Add(10 * time.Minute)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("task %s did not finish within 10m", upid)
		}
		var st struct {
			Status     string `json:"status"`
			ExitStatus string `json:"exitstatus"`
		}
		if err := c.do(http.MethodGet, path, nil, &st); err != nil {
			return err
		}
		if st.Status == "stopped" {
			if st.ExitStatus != "OK" {
				return fmt.Errorf("task %s failed: %s", upid, st.ExitStatus)
			}
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}
