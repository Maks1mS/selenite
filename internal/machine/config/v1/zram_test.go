// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
)

func TestZramApplierValidate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     ZramConfig
		wantErr string
	}{
		{
			name: "disabled passes without size",
			cfg:  ZramConfig{Enabled: mo.Some(false)},
		},
		{
			name:    "enabled without size",
			cfg:     ZramConfig{Enabled: mo.Some(true)},
			wantErr: "size is required",
		},
		{
			name: "enabled with size - ok",
			cfg:  ZramConfig{Enabled: mo.Some(true), Size: mo.Some("ram/2")},
		},
		{
			name:    "invalid compression",
			cfg:     ZramConfig{Enabled: mo.Some(true), Size: mo.Some("ram/2"), Compression: mo.Some("invalid")},
			wantErr: "invalid compression",
		},
		{
			name: "valid compression zstd",
			cfg:  ZramConfig{Enabled: mo.Some(true), Size: mo.Some("ram/2"), Compression: mo.Some("zstd")},
		},
		{
			name: "valid compression lz4hc",
			cfg:  ZramConfig{Enabled: mo.Some(true), Size: mo.Some("ram/2"), Compression: mo.Some("lz4hc")},
		},
		{
			name: "disabled with invalid compression - ok",
			cfg:  ZramConfig{Enabled: mo.Some(false), Compression: mo.Some("invalid")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewZramApplier(tt.cfg, ZramConfig{}, nil)
			err := a.Validate(ctx)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestZramApplierApply(t *testing.T) {
	ctx := context.Background()

	t.Run("no-op when config unchanged", func(t *testing.T) {
		cfg := ZramConfig{
			Enabled:      mo.Some(true),
			Size:         mo.Some("ram/2"),
			Compression:  mo.Some("zstd"),
			SwapPriority: mo.Some(100),
		}
		r := mocks.NewMockRunner(t)
		require.NoError(t, NewZramApplier(cfg, cfg, r).Apply(ctx))
	})

	t.Run("enabled - passes correct env", func(t *testing.T) {
		cfg := ZramConfig{
			Enabled:      mo.Some(true),
			Size:         mo.Some("ram/2"),
			Compression:  mo.Some("zstd"),
			SwapPriority: mo.Some(100),
		}
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, zramHelperBin, []string{"apply"},
				envMatcher(
					"SELENITE_ZRAM_ENABLED=1",
					"SELENITE_ZRAM_SIZE=ram/2",
					"SELENITE_ZRAM_COMPRESSION=zstd",
					"SELENITE_ZRAM_PRIORITY=100",
				),
			).Return(nil)
		require.NoError(t, NewZramApplier(cfg, ZramConfig{}, r).Apply(ctx))
	})

	t.Run("disabled - passes enabled=0", func(t *testing.T) {
		prev := ZramConfig{Enabled: mo.Some(true), Size: mo.Some("ram/2")}
		curr := ZramConfig{Enabled: mo.Some(false)}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, zramHelperBin, []string{"apply"},
				envMatcher("SELENITE_ZRAM_ENABLED=0"),
			).Return(nil)
		require.NoError(t, NewZramApplier(curr, prev, r).Apply(ctx))
	})

	t.Run("helper error is propagated", func(t *testing.T) {
		cfg := ZramConfig{Enabled: mo.Some(true), Size: mo.Some("ram/2")}
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, zramHelperBin, []string{"apply"}, mock.Anything).
			Return(errors.New("systemctl failed"))
		err := NewZramApplier(cfg, ZramConfig{}, r).Apply(ctx)
		require.ErrorContains(t, err, "zram-helper apply")
	})
}

func TestZramApplierRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("rollback restores previous config", func(t *testing.T) {
		prev := ZramConfig{
			Enabled:      mo.Some(true),
			Size:         mo.Some("ram/2"),
			Compression:  mo.Some("lzo"),
			SwapPriority: mo.Some(50),
		}
		curr := ZramConfig{
			Enabled:      mo.Some(true),
			Size:         mo.Some("ram/4"),
			Compression:  mo.Some("zstd"),
			SwapPriority: mo.Some(100),
		}

		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, zramHelperBin, []string{"apply"},
				envMatcher(
					"SELENITE_ZRAM_ENABLED=1",
					"SELENITE_ZRAM_SIZE=ram/2",
					"SELENITE_ZRAM_COMPRESSION=lzo",
					"SELENITE_ZRAM_PRIORITY=50",
				),
			).Return(nil)

		require.NoError(t, NewZramApplier(curr, prev, r).Rollback(ctx))
	})
}
