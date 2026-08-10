// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil"
	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
)

func TestNetworkApplierValidate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     NetworkConfig
		prev    NetworkConfig
		wantErr string
	}{
		{
			name:    "empty desired_state",
			cfg:     NetworkConfig{},
			wantErr: "desired_state",
		},
		{
			name:    "whitespace-only desired_state",
			cfg:     NetworkConfig{DesiredState: []byte("  \n\t")},
			wantErr: "desired_state",
		},
		{
			name:    "empty mapping desired_state",
			cfg:     NetworkConfig{DesiredState: []byte("{}\n")},
			wantErr: "desired_state",
		},
		{
			name:    "null desired_state",
			cfg:     NetworkConfig{DesiredState: []byte("null\n")},
			wantErr: "desired_state",
		},
		{
			name:    "non-mapping scalar desired_state",
			cfg:     NetworkConfig{DesiredState: []byte("just a string\n")},
			wantErr: "nmstate mapping",
		},
		{
			name: "cleanup mode empty new with non-empty prev - ok",
			cfg:  NetworkConfig{},
			prev: NetworkConfig{DesiredState: []byte("interfaces: []\n")},
		},
		{
			name: "empty interfaces list - ok",
			cfg: NetworkConfig{
				DesiredState: []byte("interfaces: []\n"),
			},
		},
		{
			name: "with desired_state - ok",
			cfg: NetworkConfig{
				DesiredState: []byte("interfaces:\n  - name: eth0\n    type: ethernet\n    state: up\n"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := tt.prev
			if prev.DesiredState == nil {
				prev = NetworkConfig{}
			}
			a := NewNetworkApplier(tt.cfg, prev, nil)
			err := a.Validate(ctx)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNetworkApplierApply(t *testing.T) {
	ctx := context.Background()

	desired := []byte("interfaces:\n  - name: eth0\n    type: ethernet\n    state: up\n")

	t.Run("no-op when config unchanged", func(t *testing.T) {
		cfg := NetworkConfig{DesiredState: desired}
		r := mocks.NewMockRunner(t)
		require.NoError(t, NewNetworkApplier(cfg, cfg, r).Apply(ctx))
	})

	t.Run("backup then apply when config changes", func(t *testing.T) {
		prev := NetworkConfig{DesiredState: []byte("interfaces: []\n")}
		curr := NetworkConfig{DesiredState: desired}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().
			Run(ctx, nmstateHelperBin, []string{"apply"}, mock.MatchedBy(func(opts []executil.Option) bool {
				return true
			})).
			Return(nil)

		require.NoError(t, NewNetworkApplier(curr, prev, r).Apply(ctx))
	})

	t.Run("applies desired_state via stdin", func(t *testing.T) {
		curr := NetworkConfig{DesiredState: desired}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().
			Run(ctx, nmstateHelperBin, []string{"apply"}, mock.MatchedBy(func(opts []executil.Option) bool {
				return true
			})).
			Return(nil)

		require.NoError(t, NewNetworkApplier(curr, NetworkConfig{}, r).Apply(ctx))
	})

	t.Run("backup error is propagated", func(t *testing.T) {
		curr := NetworkConfig{DesiredState: desired}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(errors.New("nmstatectl show failed"))

		err := NewNetworkApplier(curr, NetworkConfig{}, r).Apply(ctx)
		require.ErrorContains(t, err, "nmstate-helper backup")
	})

	t.Run("apply error is propagated", func(t *testing.T) {
		curr := NetworkConfig{DesiredState: desired}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().
			Run(ctx, nmstateHelperBin, []string{"apply"}, mock.Anything).
			Return(errors.New("nmstatectl apply failed"))

		err := NewNetworkApplier(curr, NetworkConfig{}, r).Apply(ctx)
		require.ErrorContains(t, err, "nmstate-helper apply")
	})

	t.Run("cleanup when section removed", func(t *testing.T) {
		prev := NetworkConfig{DesiredState: desired}
		curr := NetworkConfig{}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"cleanup"}).Return(nil)

		require.NoError(t, NewNetworkApplier(curr, prev, r).Apply(ctx))
	})

	t.Run("cleanup error is propagated", func(t *testing.T) {
		prev := NetworkConfig{DesiredState: desired}
		curr := NetworkConfig{}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"cleanup"}).Return(errors.New("nmstatectl failed"))

		err := NewNetworkApplier(curr, prev, r).Apply(ctx)
		require.ErrorContains(t, err, "nmstate-helper cleanup")
	})
}

func TestNetworkApplierRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("no-op when backup was not performed", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		require.NoError(t, NewNetworkApplier(NetworkConfig{}, NetworkConfig{}, r).Rollback(ctx))
	})

	t.Run("restores after a backup", func(t *testing.T) {
		desired := []byte("interfaces:\n  - name: eth0\n    type: ethernet\n    state: up\n")
		curr := NetworkConfig{DesiredState: desired}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().
			Run(ctx, nmstateHelperBin, []string{"apply"}, mock.Anything).
			Return(nil)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"restore"}).Return(nil)

		a := NewNetworkApplier(curr, NetworkConfig{}, r)
		require.NoError(t, a.Apply(ctx))
		require.NoError(t, a.Rollback(ctx))
	})

	t.Run("restores after cleanup failure", func(t *testing.T) {
		desired := []byte("interfaces:\n  - name: eth0\n    type: ethernet\n    state: up\n")
		prev := NetworkConfig{DesiredState: desired}
		curr := NetworkConfig{}

		r := mocks.NewMockRunner(t)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"backup"}).Return(nil)
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"cleanup"}).Return(errors.New("nmstatectl failed"))
		r.EXPECT().Run(ctx, nmstateHelperBin, []string{"restore"}).Return(nil)

		a := NewNetworkApplier(curr, prev, r)
		err := a.Apply(ctx)
		require.Error(t, err)
		require.NoError(t, a.Rollback(ctx))
	})
}

func TestNetworkApplierDeepEqual(t *testing.T) {
	// Ensure whitespace-only differences don't trigger a re-apply.
	desired := []byte("interfaces: []\n")
	desiredWS := []byte("  interfaces: []  \n")

	ctx := context.Background()
	curr := NetworkConfig{DesiredState: desired}
	prev := NetworkConfig{DesiredState: desiredWS}

	r := mocks.NewMockRunner(t)
	require.NoError(t, NewNetworkApplier(curr, prev, r).Apply(ctx))
}
