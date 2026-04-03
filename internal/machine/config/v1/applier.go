package v1

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
)

const sysctlHelperBin = "/usr/libexec/selenite-controller/sysctl-helper"

type ModuleApplier interface {
	Name() string
	Validate(ctx context.Context) error
	Apply(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Applier orchestrates atomic apply across all modules.
type Applier struct {
	modules []ModuleApplier
}

func NewApplier(cfg, prev *MachineConfig, r executil.Runner) *Applier {
	var prevSysctl map[string]string
	if prev != nil {
		prevSysctl = prev.Sysctl.Params
	}
	return &Applier{
		modules: []ModuleApplier{
			NewSysctlApplier(cfg.Sysctl.Params, prevSysctl, r),
		},
	}
}

func (a *Applier) Apply(ctx context.Context) error {
	for _, m := range a.modules {
		if err := m.Validate(ctx); err != nil {
			return fmt.Errorf("validate: %w", err)
		}
	}

	var applied []ModuleApplier
	for _, m := range a.modules {
		if err := m.Apply(ctx); err != nil {
			for i := len(applied) - 1; i >= 0; i-- {
				_ = applied[i].Rollback(ctx)
			}
			return fmt.Errorf("%s apply: %w", m.Name(), err)
		}
		applied = append(applied, m)
	}
	return nil
}

// SysctlApplier applies sysctl parameters.
type SysctlApplier struct {
	newParams  map[string]string
	prevParams map[string]string
	r          executil.Runner
}

func NewSysctlApplier(newParams, prevParams map[string]string, r executil.Runner) *SysctlApplier {
	return &SysctlApplier{newParams: newParams, prevParams: prevParams, r: r}
}

func (a *SysctlApplier) Name() string {
	return "sysctl"
}

func (a *SysctlApplier) Validate(_ context.Context) error {
	return nil
}

func (a *SysctlApplier) Apply(ctx context.Context) error {
	if err := a.runSysctlHelper(ctx, a.newParams); err != nil {
		return fmt.Errorf("sysctl apply: %w", err)
	}
	return nil
}

func (a *SysctlApplier) Rollback(ctx context.Context) error {
	if err := a.runSysctlHelper(ctx, a.prevParams); err != nil {
		return fmt.Errorf("sysctl rollback: %w", err)
	}
	return nil
}

func (a *SysctlApplier) runSysctlHelper(ctx context.Context, params map[string]string) error {
	var sb strings.Builder
	for k, v := range params {
		fmt.Fprintf(&sb, "%s=%s\n", k, v)
	}
	var stderr bytes.Buffer
	err := a.r.Run(ctx, sysctlHelperBin, []string{},
		executil.WithStdin(strings.NewReader(sb.String())),
		executil.WithStderr(&stderr),
	)
	if err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}
