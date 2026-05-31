// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package compose

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
)

type runCall struct {
	name string
	args []string
}

type mockRunner struct {
	calls []runCall
	err   error
}

func (m *mockRunner) Run(_ context.Context, name string, args []string, _ ...executil.Option) error {
	m.calls = append(m.calls, runCall{name, args})
	return m.err
}

func newTestManager(t *testing.T, runner *mockRunner) *Manager {
	t.Helper()
	return &Manager{baseDir: t.TempDir(), runner: runner}
}

func TestDeploy(t *testing.T) {
	t.Run("invalid name", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)
		err := m.Deploy(context.Background(), "Bad Name", strings.NewReader(""), io.Discard)
		require.Error(t, err)
		require.Empty(t, r.calls)
	})

	t.Run("writes compose.yml and calls up -d", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)
		content := "services:\n  web:\n    image: nginx\n"
		err := m.Deploy(context.Background(), "mystack", strings.NewReader(content), io.Discard)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(m.stackPath("mystack"), "compose.yml"))
		require.NoError(t, err)
		require.Equal(t, content, string(data))

		require.Len(t, r.calls, 1)
		require.Equal(t, ExecutorBin, r.calls[0].name)
		require.Equal(t, []string{m.stackPath("mystack"), "up", "-d"}, r.calls[0].args)
	})

	t.Run("runner error is propagated", func(t *testing.T) {
		runErr := errors.New("executor failed")
		r := &mockRunner{err: runErr}
		m := newTestManager(t, r)
		err := m.Deploy(context.Background(), "mystack", strings.NewReader(""), io.Discard)
		require.ErrorIs(t, err, runErr)
	})
}

func TestUp(t *testing.T) {
	t.Run("invalid name", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)
		err := m.Up(context.Background(), "Bad Name", io.Discard)
		require.Error(t, err)
		require.Empty(t, r.calls)
	})

	t.Run("calls up -d", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)
		err := m.Up(context.Background(), "mystack", io.Discard)
		require.NoError(t, err)
		require.Len(t, r.calls, 1)
		require.Equal(t, ExecutorBin, r.calls[0].name)
		require.Equal(t, []string{m.stackPath("mystack"), "up", "-d"}, r.calls[0].args)
	})

	t.Run("runner error is propagated", func(t *testing.T) {
		runErr := errors.New("executor failed")
		r := &mockRunner{err: runErr}
		m := newTestManager(t, r)
		err := m.Up(context.Background(), "mystack", io.Discard)
		require.ErrorIs(t, err, runErr)
	})
}

func TestRemove(t *testing.T) {
	t.Run("invalid name", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)
		err := m.Remove(context.Background(), "Bad Name", io.Discard)
		require.Error(t, err)
		require.Empty(t, r.calls)
	})

	t.Run("removes compose.yml and calls down", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)

		stacksDir := filepath.Join(m.baseDir, StacksSubdir)
		require.NoError(t, os.MkdirAll(filepath.Join(stacksDir, "mystack"), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(stacksDir, "mystack", "compose.yml"), []byte{}, 0o640)) //gosec:disable G306 -- Expected

		err := m.Remove(context.Background(), "mystack", io.Discard)
		require.NoError(t, err)

		require.Len(t, r.calls, 1)
		require.Equal(t, ExecutorBin, r.calls[0].name)
		require.Equal(t, []string{m.stackPath("mystack"), "down"}, r.calls[0].args)
		require.NoDirExists(t, filepath.Join(stacksDir, "mystack"))
	})
}

func TestDownAndPs(t *testing.T) {
	type opFunc func(m *Manager, stackName string) error

	tests := []struct {
		name     string
		op       opFunc
		wantArgs []string
	}{
		{
			name:     "Down",
			op:       func(m *Manager, s string) error { return m.Down(context.Background(), s, io.Discard) },
			wantArgs: []string{"down"},
		},
		{
			name:     "Ps",
			op:       func(m *Manager, s string) error { return m.Ps(context.Background(), s, io.Discard) },
			wantArgs: []string{"ps"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("invalid name", func(t *testing.T) {
				r := &mockRunner{}
				m := newTestManager(t, r)
				err := tt.op(m, "Bad Name")
				require.Error(t, err)
				require.Empty(t, r.calls)
			})

			t.Run("correct calls", func(t *testing.T) {
				r := &mockRunner{}
				m := newTestManager(t, r)
				err := tt.op(m, "mystack")
				require.NoError(t, err)
				require.Len(t, r.calls, 1)
				require.Equal(t, ExecutorBin, r.calls[0].name)
				require.Equal(t, append([]string{m.stackPath("mystack")}, tt.wantArgs...), r.calls[0].args)
			})
		})
	}
}

func TestLogs(t *testing.T) {
	t.Run("invalid name", func(t *testing.T) {
		r := &mockRunner{}
		m := newTestManager(t, r)
		err := m.Logs(context.Background(), "Bad Name", false, "", io.Discard)
		require.Error(t, err)
		require.Empty(t, r.calls)
	})

	tests := []struct {
		name     string
		follow   bool
		service  string
		wantArgs []string
	}{
		{"no follow, no service", false, "", []string{"logs"}},
		{"follow, no service", true, "", []string{"logs", "-f"}},
		{"no follow, with service", false, "web", []string{"logs", "web"}},
		{"follow, with service", true, "web", []string{"logs", "-f", "web"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{}
			m := newTestManager(t, r)
			err := m.Logs(context.Background(), "mystack", tt.follow, tt.service, io.Discard)
			require.NoError(t, err)
			require.Len(t, r.calls, 1)
			require.Equal(t, ExecutorBin, r.calls[0].name)
			require.Equal(t, append([]string{m.stackPath("mystack")}, tt.wantArgs...), r.calls[0].args)
		})
	}
}

func TestList(t *testing.T) {
	t.Run("stacks dir does not exist", func(t *testing.T) {
		m := newTestManager(t, &mockRunner{})
		names, err := m.List()
		require.NoError(t, err)
		require.Nil(t, names)
	})

	t.Run("returns only directories", func(t *testing.T) {
		m := newTestManager(t, &mockRunner{})
		stacksDir := filepath.Join(m.baseDir, StacksSubdir)
		require.NoError(t, os.MkdirAll(filepath.Join(stacksDir, "alpha"), 0o750))
		require.NoError(t, os.MkdirAll(filepath.Join(stacksDir, "beta"), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(stacksDir, "somefile"), []byte{}, 0o640)) //gosec:disable G306 -- Expected

		names, err := m.List()
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"alpha", "beta"}, names)
	})

	t.Run("empty stacks dir", func(t *testing.T) {
		m := newTestManager(t, &mockRunner{})
		stacksDir := filepath.Join(m.baseDir, StacksSubdir)
		require.NoError(t, os.MkdirAll(stacksDir, 0o750))

		names, err := m.List()
		require.NoError(t, err)
		require.Empty(t, names)
	})
}
