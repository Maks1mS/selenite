// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package machine

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
	config "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
)

type noopStore struct{}

func (noopStore) Load() (*config.MachineConfig, error) { return &config.MachineConfig{}, nil }
func (noopStore) Save(*config.MachineConfig) error     { return nil }

func newTestMachine(t *testing.T, runner *mocks.MockRunner) *machineManager {
	t.Helper()
	return &machineManager{
		runner: runner,
		store:  noopStore{},
	}
}

func TestDmesg(t *testing.T) {
	ctx := context.Background()

	t.Run("default", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"dmesg"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Dmesg(ctx, io.Discard, false, "", false)
		require.NoError(t, err)
	})

	t.Run("ctime", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"dmesg", "--ctime"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Dmesg(ctx, io.Discard, true, "", false)
		require.NoError(t, err)
	})

	t.Run("level", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"dmesg", "--level", "err,warn"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Dmesg(ctx, io.Discard, false, "err,warn", false)
		require.NoError(t, err)
	})

	t.Run("follow", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"dmesg", "--follow"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Dmesg(ctx, io.Discard, false, "", true)
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, mock.Anything, mock.Anything).
			Return(errors.New("dmesg failed"))

		err := newTestMachine(t, r).Dmesg(ctx, io.Discard, false, "", false)
		require.ErrorContains(t, err, "dmesg:")
	})
}

func TestLogs(t *testing.T) {
	ctx := context.Background()

	t.Run("default", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"journalctl", "--no-pager", "--output=short-iso", "-n", "100"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Logs(ctx, io.Discard, 100, false, "")
		require.NoError(t, err)
	})

	t.Run("all lines", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"journalctl", "--no-pager", "--output=short-iso"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Logs(ctx, io.Discard, -1, false, "")
		require.NoError(t, err)
	})

	t.Run("follow", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"journalctl", "--no-pager", "--output=short-iso", "-n", "50", "-f"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Logs(ctx, io.Discard, 50, true, "")
		require.NoError(t, err)
	})

	t.Run("unit filter", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, []string{"journalctl", "--no-pager", "--output=short-iso", "-n", "100", "-u", "podman.service"}, mock.Anything).
			Return(nil)

		err := newTestMachine(t, r).Logs(ctx, io.Discard, 100, false, "podman.service")
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, ExecutorBin, mock.Anything, mock.Anything).
			Return(errors.New("journalctl failed"))

		err := newTestMachine(t, r).Logs(ctx, io.Discard, 100, false, "")
		require.ErrorContains(t, err, "logs:")
	})
}

func TestApplyConfig(t *testing.T) {
	// TODO: better test implementation

	t.Run("ok", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(
				context.Background(),
				"/usr/libexec/selenite-controller/sysctl-helper",
				[]string{},
				mock.Anything,
			).
			Return(nil)

		m := newTestMachine(t, r)

		cfg := `
access:
  users:
    - name: root
      roles:
        - root
      keys:
        - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJOlcwmfZCWi/5k6bZu7GZ8LOfxwaSjf3/PkBan+UPwJ"
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: 1024
`

		err := m.ApplyConfig(context.Background(), strings.NewReader(cfg), io.Discard)
		require.NoError(t, err)
	})
}
