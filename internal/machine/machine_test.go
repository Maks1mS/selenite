// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package machine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// capturingStore records the last saved config and returns a configurable
// loaded config.
type capturingStore struct {
	loaded *config.MachineConfig
	saved  *config.MachineConfig
}

func (s *capturingStore) Load() (*config.MachineConfig, error) {
	if s.loaded == nil {
		return &config.MachineConfig{}, nil
	}
	return s.loaded, nil
}
func (s *capturingStore) Save(cfg *config.MachineConfig) error { s.saved = cfg; return nil }

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
				"/usr/libexec/selenite-controller/nmstate-helper",
				[]string{"backup"},
				mock.Anything,
			).
			Return(nil)
		r.EXPECT().
			Run(
				context.Background(),
				"/usr/libexec/selenite-controller/nmstate-helper",
				[]string{"apply"},
				mock.Anything,
			).
			Return(nil)
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
network:
  desired_state:
    interfaces:
      - name: dummy0
        type: dummy
        state: up
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: 1024
`

		err := m.ApplyConfig(context.Background(), strings.NewReader(cfg), io.Discard)
		require.NoError(t, err)
	})

	t.Run("apply without network section works", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(
				context.Background(),
				"/usr/libexec/selenite-controller/sysctl-helper",
				[]string{},
				mock.Anything,
			).
			Return(nil)

		store := &capturingStore{loaded: &config.MachineConfig{}}
		m := &machineManager{runner: r, store: store}

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

func TestProvision(t *testing.T) {
	ctx := context.Background()
	const authorizedKey = "ssh-ed25519 AAAAprovisioner"

	t.Run("adds provisioner user", func(t *testing.T) {
		store := &capturingStore{loaded: &config.MachineConfig{}}
		m := &machineManager{runner: mocks.NewMockRunner(t), store: store}

		var out bytes.Buffer
		require.NoError(t, m.Provision(ctx, &out, authorizedKey))

		require.NotNil(t, store.saved)
		require.Len(t, store.saved.Access.Users, 1)
		require.Equal(t, "provisioner", store.saved.Access.Users[0].Name)
		require.Equal(t, []string{"root"}, store.saved.Access.Users[0].Roles)
		require.Equal(t, []string{authorizedKey}, store.saved.Access.Users[0].Keys)
		require.Contains(t, out.String(), "Provisioning complete")
	})

	t.Run("save error is propagated", func(t *testing.T) {
		store := &capturingStore{loaded: &config.MachineConfig{}}
		m := &machineManager{runner: mocks.NewMockRunner(t), store: store}
		// Force save to fail by using a store that always errors.
		badStore := &failStore{inner: store}
		m.store = badStore

		err := m.Provision(ctx, io.Discard, authorizedKey)
		require.ErrorContains(t, err, "provision: save config")
	})
}

// failStore wraps a real store and always returns an error on Save.
type failStore struct {
	inner config.Store
}

func (s *failStore) Load() (*config.MachineConfig, error) {
	cfg, err := s.inner.Load()
	if err != nil {
		return nil, fmt.Errorf("failStore load: %w", err)
	}
	return cfg, nil
}
func (s *failStore) Save(*config.MachineConfig) error { return errors.New("disk full") }

func TestShowConfig(t *testing.T) {
	t.Run("outputs stored config as YAML", func(t *testing.T) {
		store := &capturingStore{loaded: &config.MachineConfig{
			Access: config.AccessConfig{
				Users: []config.User{{Name: "bob", Roles: []string{"root"}}},
			},
			Sysctl: config.SysctlConfig{Params: map[string]string{
				"net.ipv4.ip_unprivileged_port_start": "80",
			}},
		}}
		m := &machineManager{runner: mocks.NewMockRunner(t), store: store}

		var out bytes.Buffer
		require.NoError(t, m.ShowConfig(context.Background(), &out))

		require.Contains(t, out.String(), "name: bob")
		require.Contains(t, out.String(), "net.ipv4.ip_unprivileged_port_start: \"80\"")
	})
}
