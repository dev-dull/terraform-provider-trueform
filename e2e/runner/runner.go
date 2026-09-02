// Package runner executes the create -> import -> modify -> destroy terraform
// cycle against a live TrueNAS, using a locally-built provider via dev_overrides.
//
// It never mutates the tracked test-resources tree (each phase runs in a temp
// copy of the phase's *.tf files) and never writes secrets to disk: credentials
// and test parameters are injected as TF_VAR_* environment variables, so the
// placeholder terraform.tfvars files are simply not present in the temp workdir.
//
// Homelab test tooling: not part of the shipped provider.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Params carries everything the cycle needs. Creds are runtime-only.
type Params struct {
	TruenasHost        string
	TruenasAPIKey      string
	PoolName           string
	PoolDisks          []string
	TestPrefix         string
	BasePath           string
	NFSAllowedNetwork  string
	StaticRouteGateway string
	AppVersion         string
}

// Runner holds a built provider + dev-override config for one or more cycles.
type Runner struct {
	repoRoot  string
	testDir   string
	binDir    string // temp: holds the built provider binary + CLI config
	cliConfig string
	tfvars    []string
}

// New builds the provider into a temp dir and writes a dev_overrides CLI config.
// Call Close to remove both.
func New(repoRoot string, p Params) (*Runner, error) {
	binDir, err := os.MkdirTemp("", "e2e-provider-*")
	if err != nil {
		return nil, err
	}

	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "terraform-provider-trueform"), ".")
	build.Dir = repoRoot
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		_ = os.RemoveAll(binDir)
		return nil, fmt.Errorf("build provider: %w", err)
	}

	cliConfig := filepath.Join(binDir, "dev.tfrc")
	rc := fmt.Sprintf(`provider_installation {
  dev_overrides {
    "registry.terraform.io/trueform/trueform" = %q
  }
  direct {}
}
`, binDir)
	if err := os.WriteFile(cliConfig, []byte(rc), 0o600); err != nil {
		_ = os.RemoveAll(binDir)
		return nil, err
	}

	// TF_VAR_* superset covering create/import (nfs_allowed_network, singular)
	// and modify (nfs_allowed_networks, plural; pool_name; base_path). Undeclared
	// vars are ignored per config, so a superset is safe.
	base := p.BasePath
	if base == "" {
		base = "/mnt/" + p.PoolName
	}
	vars := map[string]string{
		"truenas_host":         p.TruenasHost,
		"truenas_api_key":      p.TruenasAPIKey,
		"truenas_verify_ssl":   "false",
		"pool_name":            p.PoolName,
		"test_prefix":          p.TestPrefix,
		"base_path":            base,
		"pool_disks":           jsonList(p.PoolDisks),
		"nfs_allowed_network":  p.NFSAllowedNetwork,
		"nfs_allowed_networks": jsonList([]string{p.NFSAllowedNetwork}),
		"static_route_gateway": p.StaticRouteGateway,
		"test_app_version":     p.AppVersion,
	}
	var tfvars []string
	for k, v := range vars {
		if v == "" {
			continue // let the config's own default apply rather than force ""
		}
		tfvars = append(tfvars, "TF_VAR_"+k+"="+v)
	}

	return &Runner{
		repoRoot:  repoRoot,
		testDir:   filepath.Join(repoRoot, "test-resources"),
		binDir:    binDir,
		cliConfig: cliConfig,
		tfvars:    tfvars,
	}, nil
}

func (r *Runner) Close() error { return os.RemoveAll(r.binDir) }

func (r *Runner) env() []string {
	e := append(os.Environ(), r.tfvars...)
	e = append(e, "TF_CLI_CONFIG_FILE="+r.cliConfig, "TF_IN_AUTOMATION=1")
	return e
}

// terraform runs a terraform subcommand, streaming output, and returns its exit
// code (0 on success). A non-ExitError (e.g. terraform missing) is returned as err.
func (r *Runner) terraform(ctx context.Context, workdir string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = workdir
	cmd.Env = r.env()
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

func (r *Runner) capture(ctx context.Context, workdir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = workdir
	cmd.Env = r.env()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, os.Stderr
	err := cmd.Run()
	return buf.Bytes(), err
}

// RunCycle runs create -> import -> (assert clean plan) -> modify -> destroy ->
// (assert empty state). On any failure the temp workdir is preserved and its
// path returned in the error for inspection.
func (r *Runner) RunCycle(ctx context.Context) error {
	work, err := os.MkdirTemp("", "e2e-cycle-*")
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if keep {
			fmt.Printf("  (workdir preserved for inspection: %s)\n", work)
		} else {
			_ = os.RemoveAll(work)
		}
	}()
	fail := func(format string, a ...interface{}) error {
		keep = true
		return fmt.Errorf(format, a...)
	}

	createDir := filepath.Join(work, "create")
	importDir := filepath.Join(work, "import")
	modifyDir := filepath.Join(work, "modify")
	for _, d := range []string{createDir, importDir, modifyDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	// CREATE
	fmt.Println("  [create] apply")
	if err := r.copyPhaseTF("create", createDir); err != nil {
		return err
	}
	if code, err := r.terraform(ctx, createDir, "apply", "-auto-approve", "-input=false"); err != nil {
		return err
	} else if code != 0 {
		return fail("create apply failed (exit %d)", code)
	}

	// IMPORT (IDs derived from create state)
	fmt.Println("  [import] generate imports.tf from create state, apply, assert clean plan")
	if err := r.copyPhaseTF("import", importDir); err != nil {
		return err
	}
	if err := r.genImports(ctx, createDir, importDir); err != nil {
		return fail("generate imports: %v", err)
	}
	if code, err := r.terraform(ctx, importDir, "apply", "-auto-approve", "-input=false"); err != nil {
		return err
	} else if code != 0 {
		return fail("import apply failed (exit %d)", code)
	}
	code, err := r.terraform(ctx, importDir, "plan", "-detailed-exitcode", "-input=false")
	if err != nil {
		return err
	}
	switch code {
	case 0:
		// clean, no changes — the assertion we want
	case 2:
		return fail("import plan is NOT clean: drift detected after import")
	default:
		return fail("import plan errored (exit %d)", code)
	}

	// MODIFY (continues from import's state)
	fmt.Println("  [modify] apply")
	if err := r.copyPhaseTF("modify", modifyDir); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(importDir, "terraform.tfstate"), filepath.Join(modifyDir, "terraform.tfstate")); err != nil {
		return fail("copy state import->modify: %v", err)
	}
	if code, err := r.terraform(ctx, modifyDir, "apply", "-auto-approve", "-input=false"); err != nil {
		return err
	} else if code != 0 {
		return fail("modify apply failed (exit %d)", code)
	}

	// DESTROY
	fmt.Println("  [destroy] destroy, assert empty state")
	if code, err := r.terraform(ctx, modifyDir, "destroy", "-auto-approve", "-input=false"); err != nil {
		return err
	} else if code != 0 {
		return fail("destroy failed (exit %d)", code)
	}
	out, err := r.capture(ctx, modifyDir, "state", "list")
	if err != nil {
		return fail("state list: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fail("state not empty after destroy:\n%s", out)
	}

	return nil
}

// copyPhaseTF copies a phase dir's *.tf files into dst, excluding any tracked
// imports.tf (the import phase's is generated). tfvars are intentionally not
// copied, so TF_VAR_* values take effect.
func (r *Runner) copyPhaseTF(phase, dst string) error {
	src := filepath.Join(r.testDir, phase)
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".tf") || name == "imports.tf" {
			continue
		}
		if err := copyFile(filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
			return err
		}
	}
	return nil
}

// genImports reads the create phase's state and writes an imports.tf for every
// created resource except trueform_service_docker (which the import config, like
// the tracked one, does not manage).
func (r *Runner) genImports(ctx context.Context, createDir, importDir string) error {
	out, err := r.capture(ctx, createDir, "show", "-json")
	if err != nil {
		return fmt.Errorf("terraform show: %w", err)
	}
	var show struct {
		Values struct {
			RootModule struct {
				Resources []struct {
					Address string `json:"address"`
					Type    string `json:"type"`
					Values  struct {
						// This provider types some ids as numbers (cronjob, user,
						// iscsi_*) and others as strings (dataset path, app name),
						// so decode raw and coerce. Import ids are always strings.
						ID json.RawMessage `json:"id"`
					} `json:"values"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal(out, &show); err != nil {
		return fmt.Errorf("parse show json: %w", err)
	}

	var b strings.Builder
	b.WriteString("# Generated by e2e runner from create-phase state. Do not edit.\n\n")
	n := 0
	for _, res := range show.Values.RootModule.Resources {
		if res.Type == "trueform_service_docker" {
			continue
		}
		id := strings.Trim(string(res.Values.ID), `"`)
		if id == "" || id == "null" {
			return fmt.Errorf("resource %s has no id in create state", res.Address)
		}
		fmt.Fprintf(&b, "import {\n  to = %s\n  id = %q\n}\n\n", res.Address, id)
		n++
	}
	if n == 0 {
		return fmt.Errorf("no importable resources found in create state")
	}
	return os.WriteFile(filepath.Join(importDir, "imports.tf"), []byte(b.String()), 0o600)
}

func jsonList(items []string) string {
	b, _ := json.Marshal(items)
	return string(b)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}
