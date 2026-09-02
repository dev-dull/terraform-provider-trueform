// Package truenas is a thin harness-side wrapper over the provider's
// internal/client. It adds the operations the e2e harness needs around a
// snapshot rollback — waiting for the guest to come back, and asserting it is
// healthy and the expected major version — reusing the same WebSocket client
// (and version detection) the provider itself uses.
//
// Homelab test tooling: not part of the shipped provider.
package truenas

import (
	"context"
	"fmt"
	"time"

	"github.com/trueform/terraform-provider-trueform/internal/client"
)

// Client wraps a connected internal/client.Client.
type Client struct {
	c    *client.Client
	Host string
}

func newClient(host, apiKey string) *client.Client {
	return client.NewClient(&client.Config{
		Host:      host,
		APIKey:    apiKey,
		VerifySSL: false,
		Timeout:   30 * time.Second,
	})
}

// WaitForReady polls the TrueNAS API until it answers or the timeout elapses,
// then returns a connected client. Snapshots here are disk-only (no RAM), so a
// rollback is followed by a full guest boot; expect a minute or more. The caller
// owns Close().
func WaitForReady(ctx context.Context, host, apiKey string, timeout time.Duration) (*Client, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c := newClient(host, apiKey)
		err := c.Connect(ctx)
		if err == nil {
			var v string
			if err = c.Call(ctx, "system.version", []interface{}{}, &v); err == nil {
				return &Client{c: c, Host: host}, nil
			}
		}
		// Close on every path that does not return, so a boot's worth of failed
		// attempts cannot leak connections (Close is safe after a failed Connect).
		lastErr = err
		_ = c.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return nil, fmt.Errorf("truenas at %s not ready after %s: %w", host, timeout, lastErr)
}

// Close releases the underlying connection.
func (t *Client) Close() error { return t.c.Close() }

// MajorVersion returns the detected TrueNAS major version (populated during
// Connect), or 0 if unknown.
func (t *Client) MajorVersion() int { return t.c.ServerMajorVersion() }

// Version returns the raw system.version string.
func (t *Client) Version(ctx context.Context) (string, error) {
	var v string
	if err := t.c.Call(ctx, "system.version", []interface{}{}, &v); err != nil {
		return "", err
	}
	return v, nil
}

// HealthCheck asserts the server is the expected major version and that no
// imported pool is in a non-ONLINE state. A freshly-restored golden may have no
// user pools at all (test pools are created and destroyed within a cycle); that
// is healthy. Pass expectedMajor=0 to skip the version assertion.
func (t *Client) HealthCheck(ctx context.Context, expectedMajor int) error {
	if expectedMajor > 0 {
		if got := t.c.ServerMajorVersion(); got != expectedMajor {
			return fmt.Errorf("version mismatch: expected TrueNAS %d, detected major %d", expectedMajor, got)
		}
	}
	var pools []map[string]interface{}
	if err := t.c.Call(ctx, "pool.query", []interface{}{}, &pools); err != nil {
		return fmt.Errorf("pool.query: %w", err)
	}
	for _, p := range pools {
		// TrueNAS returns pool.status as a string ("ONLINE"/"DEGRADED"/...). A
		// non-string or missing value coerces to "" and is treated as "no signal"
		// rather than failed, to avoid false negatives on an unexpected shape;
		// Phase 2b validates the live pool.query shape once a pool exists.
		status, _ := p["status"].(string)
		if status != "" && status != "ONLINE" {
			name, _ := p["name"].(string)
			return fmt.Errorf("pool %q is %s (not ONLINE)", name, status)
		}
	}
	return nil
}

// PoolCount returns the number of imported (user) pools, for reporting.
func (t *Client) PoolCount(ctx context.Context) (int, error) {
	var pools []map[string]interface{}
	if err := t.c.Call(ctx, "pool.query", []interface{}{}, &pools); err != nil {
		return 0, err
	}
	return len(pools), nil
}
