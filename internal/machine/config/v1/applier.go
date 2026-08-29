// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	gossh "golang.org/x/crypto/ssh"

	"github.com/goccy/go-yaml"

	"altlinux.space/alt-atomic/selenite/internal/executil"
)

const (
	sysctlHelperBin  = "/usr/libexec/selenite-controller/sysctl-helper"
	netbirdHelperBin = "/usr/libexec/selenite-controller/netbird-helper"
	alloyHelperBin   = "/usr/libexec/selenite-controller/alloy-helper"
	nmstateHelperBin = "/usr/libexec/selenite-controller/nmstate-helper"
	zramHelperBin    = "/usr/libexec/selenite-controller/zram-helper"
)

type ModuleApplier interface {
	Name() string
	Validate(ctx context.Context) error
	Apply(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Applier orchestrates atomic apply across all modules.
type Applier struct {
	modules []ModuleApplier
	prev    *MachineConfig
	cfg     *MachineConfig
	store   Store
}

func NewApplier(cfg, prev *MachineConfig, r executil.Runner, s Store) *Applier {
	var prevSysctl map[string]string
	if prev != nil {
		prevSysctl = prev.Sysctl.Params
	}
	var prevNetbird NetbirdConfig
	if prev != nil {
		prevNetbird = prev.Netbird
	}
	var prevMonitoring MonitoringConfig
	if prev != nil {
		prevMonitoring = prev.Monitoring
	}
	var prevNetwork NetworkConfig
	if prev != nil {
		prevNetwork = prev.Network
	}
	var prevMemory MemoryConfig
	if prev != nil {
		prevMemory = prev.Memory
	}

	modules := []ModuleApplier{
		NewAccessApplier(cfg.Access),
		NewSysctlApplier(cfg.Sysctl.Params, prevSysctl, r),
		NewNetbirdApplier(cfg.Netbird, prevNetbird, r),
		NewMonitoringApplier(cfg.Monitoring, prevMonitoring, r),
		NewZramApplier(cfg.Memory.Zram, prevMemory.Zram, r),
	}
	// Network is managed when either the current or previous config has a
	// desired state. When the current config drops the section we run a
	// cleanup that restores the baseline captured before the first apply.
	if len(bytes.TrimSpace(cfg.Network.DesiredState)) > 0 || len(bytes.TrimSpace(prev.Network.DesiredState)) > 0 {
		modules = append(modules, NewNetworkApplier(cfg.Network, prevNetwork, r))
	}
	return &Applier{
		modules: modules,
		prev:    prev,
		cfg:     cfg,
		store:   s,
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
			_ = m.Rollback(ctx)
			for i := len(applied) - 1; i >= 0; i-- {
				_ = applied[i].Rollback(ctx)
			}
			return fmt.Errorf("%s apply: %w", m.Name(), err)
		}
		applied = append(applied, m)
	}

	if err := a.store.Save(a.cfg); err != nil {
		for i := len(applied) - 1; i >= 0; i-- {
			_ = applied[i].Rollback(ctx)
		}
		return fmt.Errorf("persist config: %w", err)
	}

	return nil
}

type AccessApplier struct {
	newParams AccessConfig
}

func NewAccessApplier(newParams AccessConfig) *AccessApplier {
	return &AccessApplier{newParams}
}

func (a *AccessApplier) Name() string {
	return "access"
}

func (a *AccessApplier) Validate(_ context.Context) error {
	if len(a.newParams.Users) == 0 {
		return errors.New("users can't be empty")
	}
	if err := a.validateAuthGuard(); err != nil {
		return err
	}
	return a.validateRoot()
}

func (a *AccessApplier) validateAuthGuard() error {
	g := a.newParams.AuthGuard
	if !g.Enabled {
		return nil
	}
	if g.MaxRetries < 1 {
		return errors.New("auth_guard.max_retries must be at least 1")
	}
	if g.Window <= 0 {
		return errors.New("auth_guard.window must be positive")
	}
	if g.BanDuration <= 0 {
		return errors.New("auth_guard.ban_duration must be positive")
	}
	if g.MaxTrackedIPs < 1 {
		return errors.New("auth_guard.max_tracked_ips must be at least 1")
	}
	return nil
}

func (a *AccessApplier) validateRoot() error {
	hasRoot := false
	hasRootWithKeys := false

	for _, user := range a.newParams.Users {
		if slices.Contains(user.Roles, "root") {
			hasRoot = true
			if len(user.Keys) > 0 {
				hasRootWithKeys = true
			}
		}
		for _, keyStr := range user.Keys {
			_, _, _, _, err := gossh.ParseAuthorizedKey([]byte(keyStr))
			if err != nil {
				return fmt.Errorf("incorrect key: %w", err)
			}
		}
	}

	if !hasRoot {
		return errors.New("at least one user must have the root role")
	}
	if !hasRootWithKeys {
		return errors.New("at least one root user must have keys")
	}

	return nil
}

func (a *AccessApplier) Apply(ctx context.Context) error {
	return nil
}

func (a *AccessApplier) Rollback(ctx context.Context) error {
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

type NetbirdApplier struct {
	newParams  NetbirdConfig
	prevParams NetbirdConfig
	backupDone bool
	r          executil.Runner
}

func NewNetbirdApplier(newParams, prevParams NetbirdConfig, r executil.Runner) *NetbirdApplier {
	return &NetbirdApplier{newParams: newParams, prevParams: prevParams, r: r}
}

func (a *NetbirdApplier) Name() string {
	return "netbird"
}

func (a *NetbirdApplier) Validate(_ context.Context) error {
	if a.newParams.Enabled {
		if a.newParams.SetupKey == "" {
			return errors.New("setup_key is empty")
		}
	}
	return nil
}

func (a *NetbirdApplier) Apply(ctx context.Context) error {
	if reflect.DeepEqual(a.newParams, a.prevParams) {
		return nil
	}

	if err := a.r.Run(ctx, netbirdHelperBin, []string{"backup"}); err != nil {
		return fmt.Errorf("netbird-helper backup: %w", err)
	}
	a.backupDone = true

	needsClear := a.newParams.Enabled && a.newParams.SetupKey != a.prevParams.SetupKey
	clearState := "0"
	if needsClear {
		clearState = "1"
	}

	enabled := "0"
	if a.newParams.Enabled {
		enabled = "1"
	}

	if err := a.r.Run(
		ctx,
		netbirdHelperBin,
		[]string{"apply"},
		executil.WithEnv("SELENITE_NETBIRD_HELPER_ENABLED", enabled),
		executil.WithEnv("SELENITE_NETBIRD_HELPER_SETUP_KEY", a.newParams.SetupKey),
		executil.WithEnv("SELENITE_NETBIRD_HELPER_CLEAR_STATE", clearState),
	); err != nil {
		return fmt.Errorf("netbird-helper apply: %w", err)
	}
	return nil
}

func (a *NetbirdApplier) Rollback(ctx context.Context) error {
	if !a.backupDone {
		return nil
	}

	enabled := "0"
	if a.prevParams.Enabled {
		enabled = "1"
	}

	if err := a.r.Run(
		ctx,
		netbirdHelperBin,
		[]string{"restore"},
		executil.WithEnv("SELENITE_NETBIRD_HELPER_ENABLED", enabled),
	); err != nil {
		return fmt.Errorf("netbird-helper restore: %w", err)
	}
	return nil
}

type MonitoringApplier struct {
	newParams  MonitoringConfig
	prevParams MonitoringConfig
	r          executil.Runner
}

func NewMonitoringApplier(newParams, prevParams MonitoringConfig, r executil.Runner) *MonitoringApplier {
	return &MonitoringApplier{newParams: newParams, prevParams: prevParams, r: r}
}

func (a *MonitoringApplier) Name() string {
	return "monitoring"
}

func (a *MonitoringApplier) Validate(_ context.Context) error {
	if !a.newParams.Enabled {
		return nil
	}
	if a.newParams.Prometheus.Enabled && a.newParams.Prometheus.Endpoint == "" {
		return errors.New("monitoring.prometheus.endpoint is required when prometheus is enabled")
	}
	if a.newParams.Loki.Enabled && a.newParams.Loki.Endpoint == "" {
		return errors.New("monitoring.loki.endpoint is required when loki is enabled")
	}
	if !a.newParams.Prometheus.Enabled && !a.newParams.Loki.Enabled {
		return errors.New("at least one of monitoring.prometheus or monitoring.loki must be enabled")
	}
	return nil
}

func (a *MonitoringApplier) Apply(ctx context.Context) error {
	if reflect.DeepEqual(a.newParams, a.prevParams) {
		return nil
	}
	return a.runHelper(ctx, a.newParams)
}

func (a *MonitoringApplier) Rollback(ctx context.Context) error {
	return a.runHelper(ctx, a.prevParams)
}

func (a *MonitoringApplier) runHelper(ctx context.Context, p MonitoringConfig) error {
	enabled := boolEnv(p.Enabled)
	prometheusEnabled := boolEnv(p.Enabled && p.Prometheus.Enabled)
	lokiEnabled := boolEnv(p.Enabled && p.Loki.Enabled)

	if err := a.r.Run(
		ctx,
		alloyHelperBin,
		[]string{"apply"},
		executil.WithEnv("SELENITE_ALLOY_HELPER_ENABLED", enabled),
		executil.WithEnv("SELENITE_ALLOY_HELPER_PROMETHEUS_ENABLED", prometheusEnabled),
		executil.WithEnv("SELENITE_ALLOY_HELPER_PROMETHEUS_ENDPOINT", p.Prometheus.Endpoint),
		executil.WithEnv("SELENITE_ALLOY_HELPER_PROMETHEUS_USERNAME", p.Prometheus.Auth.Username),
		executil.WithEnv("SELENITE_ALLOY_HELPER_PROMETHEUS_PASSWORD", p.Prometheus.Auth.Password),
		executil.WithEnv("SELENITE_ALLOY_HELPER_LOKI_ENABLED", lokiEnabled),
		executil.WithEnv("SELENITE_ALLOY_HELPER_LOKI_ENDPOINT", p.Loki.Endpoint),
		executil.WithEnv("SELENITE_ALLOY_HELPER_LOKI_USERNAME", p.Loki.Auth.Username),
		executil.WithEnv("SELENITE_ALLOY_HELPER_LOKI_PASSWORD", p.Loki.Auth.Password),
	); err != nil {
		return fmt.Errorf("alloy-helper apply: %w", err)
	}
	return nil
}

type ZramApplier struct {
	newParams  ZramConfig
	prevParams ZramConfig
	r          executil.Runner
}

func NewZramApplier(newParams, prevParams ZramConfig, r executil.Runner) *ZramApplier {
	return &ZramApplier{newParams: newParams, prevParams: prevParams, r: r}
}

func (a *ZramApplier) Name() string { return "zram" }

func (a *ZramApplier) Validate(_ context.Context) error {
	if !a.newParams.Enabled {
		return nil
	}
	if a.newParams.Size == "" {
		return errors.New("memory.zram.size is required when enabled")
	}
	validCompressions := []string{"zstd", "lzo", "lz4", "lzo-rle", "lz4hc"}
	if a.newParams.Compression != "" && !slices.Contains(validCompressions, a.newParams.Compression) {
		return fmt.Errorf("invalid compression algorithm: %s", a.newParams.Compression)
	}
	return nil
}

func (a *ZramApplier) Apply(ctx context.Context) error {
	if reflect.DeepEqual(a.newParams, a.prevParams) {
		return nil
	}
	return a.runHelper(ctx, a.newParams)
}

func (a *ZramApplier) Rollback(ctx context.Context) error {
	return a.runHelper(ctx, a.prevParams)
}

func (a *ZramApplier) runHelper(ctx context.Context, p ZramConfig) error {
	if err := a.r.Run(
		ctx,
		zramHelperBin,
		[]string{"apply"},
		executil.WithEnv("SELENITE_ZRAM_ENABLED", boolEnv(p.Enabled)),
		executil.WithEnv("SELENITE_ZRAM_SIZE", p.Size),
		executil.WithEnv("SELENITE_ZRAM_COMPRESSION", p.Compression),
		executil.WithEnv("SELENITE_ZRAM_PRIORITY", fmt.Sprintf("%d", p.SwapPriority)),
	); err != nil {
		return fmt.Errorf("zram-helper apply: %w", err)
	}
	return nil
}

func boolEnv(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// NetworkApplier applies the desired network state via nmstate.
type NetworkApplier struct {
	newParams  NetworkConfig
	prevParams NetworkConfig
	r          executil.Runner

	backupDone bool
}

func NewNetworkApplier(newParams, prevParams NetworkConfig, r executil.Runner) *NetworkApplier {
	return &NetworkApplier{newParams: newParams, prevParams: prevParams, r: r}
}

func (a *NetworkApplier) Name() string {
	return "network"
}

func (a *NetworkApplier) Validate(_ context.Context) error {
	if len(bytes.TrimSpace(a.newParams.DesiredState)) == 0 {
		if len(bytes.TrimSpace(a.prevParams.DesiredState)) == 0 {
			return errors.New("network.desired_state is required")
		}
		// The section was removed from the config (cleanup mode), so there
		// is nothing new to validate. The baseline-driven cleanup is handled
		// by Apply.
		return nil
	}
	// nmstate desired state is always a YAML mapping (interfaces, routes, ...
	// top-level keys). An empty mapping or a non-mapping scalar is not a
	// meaningful network configuration and is rejected.
	var state map[string]any
	if err := yaml.Unmarshal(a.newParams.DesiredState, &state); err != nil {
		return fmt.Errorf("network.desired_state is not a valid nmstate mapping: %w", err)
	}
	if len(state) == 0 {
		return errors.New("network.desired_state must not be empty")
	}
	return nil
}

func (a *NetworkApplier) Apply(ctx context.Context) error {
	if bytes.Equal(bytes.TrimSpace(a.newParams.DesiredState), bytes.TrimSpace(a.prevParams.DesiredState)) {
		return nil
	}

	if err := a.r.Run(ctx, nmstateHelperBin, []string{"backup"}); err != nil {
		return fmt.Errorf("nmstate-helper backup: %w", err)
	}
	a.backupDone = true

	if len(bytes.TrimSpace(a.newParams.DesiredState)) == 0 {
		if err := a.r.Run(ctx, nmstateHelperBin, []string{"cleanup"}); err != nil {
			return fmt.Errorf("nmstate-helper cleanup: %w", err)
		}
		return nil
	}

	if err := a.r.Run(
		ctx,
		nmstateHelperBin,
		[]string{"apply"},
		executil.WithStdin(bytes.NewReader(a.newParams.DesiredState)),
	); err != nil {
		return fmt.Errorf("nmstate-helper apply: %w", err)
	}
	return nil
}

func (a *NetworkApplier) Rollback(ctx context.Context) error {
	if !a.backupDone {
		return nil
	}
	if err := a.r.Run(ctx, nmstateHelperBin, []string{"restore"}); err != nil {
		return fmt.Errorf("nmstate-helper restore: %w", err)
	}
	return nil
}
