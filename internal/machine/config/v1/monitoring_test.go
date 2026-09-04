// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil"
	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
)

// envMatcher returns a mock.MatchedBy that applies all opts and checks that each
// of the given "KEY=VALUE" strings is present in the resulting cmd.Env.
func envMatcher(pairs ...string) interface{} {
	return mock.MatchedBy(func(opts []executil.Option) bool {
		cmd := &exec.Cmd{}
		for _, o := range opts {
			o(cmd)
		}
		for _, pair := range pairs {
			if !slices.Contains(cmd.Env, pair) {
				return false
			}
		}
		return true
	})
}

func TestMonitoringApplierValidate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     MonitoringConfig
		wantErr string
	}{
		{
			name: "disabled passes without any subsystem",
			cfg:  MonitoringConfig{Enabled: mo.Some(false)},
		},
		{
			name:    "enabled with no subsystem enabled",
			cfg:     MonitoringConfig{Enabled: mo.Some(true)},
			wantErr: "at least one",
		},
		{
			name: "prometheus enabled without endpoint",
			cfg: MonitoringConfig{
				Enabled:    mo.Some(true),
				Prometheus: PrometheusConfig{Enabled: mo.Some(true)},
			},
			wantErr: "endpoint",
		},
		{
			name: "loki enabled without endpoint",
			cfg: MonitoringConfig{
				Enabled: mo.Some(true),
				Loki:    LokiConfig{Enabled: mo.Some(true)},
			},
			wantErr: "endpoint",
		},
		{
			name: "prometheus only - ok",
			cfg: MonitoringConfig{
				Enabled: mo.Some(true),
				Prometheus: PrometheusConfig{
					Enabled:  mo.Some(true),
					Endpoint: mo.Some("http://mimir:9090/api/v1/write"),
				},
			},
		},
		{
			name: "loki only - ok",
			cfg: MonitoringConfig{
				Enabled: mo.Some(true),
				Loki: LokiConfig{
					Enabled:  mo.Some(true),
					Endpoint: mo.Some("http://loki:3100/loki/api/v1/push"),
				},
			},
		},
		{
			name: "both enabled - ok",
			cfg: MonitoringConfig{
				Enabled: mo.Some(true),
				Prometheus: PrometheusConfig{
					Enabled:  mo.Some(true),
					Endpoint: mo.Some("http://mimir:9090/api/v1/write"),
				},
				Loki: LokiConfig{
					Enabled:  mo.Some(true),
					Endpoint: mo.Some("http://loki:3100/loki/api/v1/push"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewMonitoringApplier(tt.cfg, MonitoringConfig{}, nil)
			err := a.Validate(ctx)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMonitoringApplierApply(t *testing.T) {
	ctx := context.Background()

	t.Run("no-op when config unchanged", func(t *testing.T) {
		cfg := MonitoringConfig{
			Enabled:    mo.Some(true),
			Prometheus: PrometheusConfig{Enabled: mo.Some(true), Endpoint: mo.Some("http://mimir:9090/api/v1/write")},
		}
		r := mocks.NewMockRunner(t)
		// runner must not be called
		require.NoError(t, NewMonitoringApplier(cfg, cfg, r).Apply(ctx))
	})

	t.Run("disabled - passes enabled=0 to helper", func(t *testing.T) {
		prev := MonitoringConfig{
			Enabled:    mo.Some(true),
			Prometheus: PrometheusConfig{Enabled: mo.Some(true), Endpoint: mo.Some("x")},
		}
		curr := MonitoringConfig{Enabled: mo.Some(false)}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, alloyHelperBin, []string{"apply"},
				envMatcher("SELENITE_ALLOY_HELPER_ENABLED=0"),
			).Return(nil)

		require.NoError(t, NewMonitoringApplier(curr, prev, r).Apply(ctx))
	})

	t.Run("prometheus only - passes correct env", func(t *testing.T) {
		cfg := MonitoringConfig{
			Enabled: mo.Some(true),
			Prometheus: PrometheusConfig{
				Enabled:  mo.Some(true),
				Endpoint: mo.Some("http://mimir:9090/api/v1/write"),
				Auth:     MonitoringAuth{Username: mo.Some("user"), Password: mo.Some("pass")},
			},
		}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, alloyHelperBin, []string{"apply"},
				envMatcher(
					"SELENITE_ALLOY_HELPER_ENABLED=1",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_ENABLED=1",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_ENDPOINT=http://mimir:9090/api/v1/write",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_USERNAME=user",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_PASSWORD=pass",
					"SELENITE_ALLOY_HELPER_LOKI_ENABLED=0",
				),
			).Return(nil)

		require.NoError(t, NewMonitoringApplier(cfg, MonitoringConfig{}, r).Apply(ctx))
	})

	t.Run("loki only - passes correct env", func(t *testing.T) {
		cfg := MonitoringConfig{
			Enabled: mo.Some(true),
			Loki: LokiConfig{
				Enabled:  mo.Some(true),
				Endpoint: mo.Some("http://loki:3100/loki/api/v1/push"),
			},
		}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, alloyHelperBin, []string{"apply"},
				envMatcher(
					"SELENITE_ALLOY_HELPER_ENABLED=1",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_ENABLED=0",
					"SELENITE_ALLOY_HELPER_LOKI_ENABLED=1",
					"SELENITE_ALLOY_HELPER_LOKI_ENDPOINT=http://loki:3100/loki/api/v1/push",
				),
			).Return(nil)

		require.NoError(t, NewMonitoringApplier(cfg, MonitoringConfig{}, r).Apply(ctx))
	})

	t.Run("helper error is propagated", func(t *testing.T) {
		cfg := MonitoringConfig{
			Enabled:    mo.Some(true),
			Prometheus: PrometheusConfig{Enabled: mo.Some(true), Endpoint: mo.Some("http://mimir:9090/api/v1/write")},
		}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, alloyHelperBin, []string{"apply"}, mock.Anything).
			Return(errors.New("systemctl failed"))

		err := NewMonitoringApplier(cfg, MonitoringConfig{}, r).Apply(ctx)
		require.ErrorContains(t, err, "alloy-helper apply")
	})
}

func TestMonitoringApplierRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("rollback restores previous config", func(t *testing.T) {
		prev := MonitoringConfig{
			Enabled: mo.Some(true),
			Prometheus: PrometheusConfig{
				Enabled:  mo.Some(true),
				Endpoint: mo.Some("http://old:9090/api/v1/write"),
			},
		}
		curr := MonitoringConfig{
			Enabled: mo.Some(true),
			Loki:    LokiConfig{Enabled: mo.Some(true), Endpoint: mo.Some("http://loki:3100/loki/api/v1/push")},
		}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, alloyHelperBin, []string{"apply"},
				envMatcher(
					"SELENITE_ALLOY_HELPER_ENABLED=1",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_ENABLED=1",
					"SELENITE_ALLOY_HELPER_PROMETHEUS_ENDPOINT=http://old:9090/api/v1/write",
					"SELENITE_ALLOY_HELPER_LOKI_ENABLED=0",
				),
			).Return(nil)

		require.NoError(t, NewMonitoringApplier(curr, prev, r).Rollback(ctx))
	})
}
