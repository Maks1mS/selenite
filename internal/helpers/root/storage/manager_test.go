// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/helpers/root/storage/mocks"
)

var testMount = Mount{
	Type:    "nfs",
	Source:  "nas.local:/volume1/video",
	Target:  "/var/mnt/video",
	Options: "ro,soft",
}

const testUnit = "var-mnt-video.mount"

// testManager wires a Manager to a scratch root and a mock Systemctl.
type testManager struct {
	*Manager
	sys  *mocks.MockSystemctl
	root string
	warn bytes.Buffer
}

func newTestManager(t *testing.T) *testManager {
	t.Helper()
	tm := &testManager{
		sys:  mocks.NewMockSystemctl(t),
		root: t.TempDir(),
	}
	// The unit directory always exists on a real host.
	require.NoError(t, os.MkdirAll(filepath.Join(tm.root, unitDir), 0o755)) //nolint:gosec // scratch files in a test sandbox
	tm.Manager = &Manager{
		Root:       tm.root,
		UnitDir:    unitDir,
		DropinDir:  dropinDir,
		GateScript: gateScript,
		Systemctl:  tm.sys,
		Warn:       &tm.warn,
	}
	return tm
}

func (tm *testManager) unitPath(name string) string {
	return filepath.Join(tm.root, unitDir, name)
}

func (tm *testManager) dropinPath() string {
	return filepath.Join(tm.root, dropinDir, dropinName)
}

func (tm *testManager) writeUnit(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(tm.unitPath(name)), 0o755))     //nolint:gosec // scratch files in a test sandbox
	require.NoError(t, os.WriteFile(tm.unitPath(name), []byte(content), 0o644)) //nolint:gosec // scratch files in a test sandbox
}

func (tm *testManager) readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // scratch files in a test sandbox
	require.NoError(t, err)
	return string(b)
}

func TestManagerApply(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("writes units and drop-in, enables and restarts", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.sys.EXPECT().DaemonReload(mock.Anything).Return(nil)
		tm.sys.EXPECT().Enable(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().Restart(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().ExecutorDaemonReload(mock.Anything).Return(nil)

		require.NoError(t, tm.Apply(ctx, []Mount{testMount}))

		require.Equal(t, UnitContent(testMount), tm.readFile(t, tm.unitPath(testUnit)))
		require.DirExists(t, filepath.Join(tm.root, testMount.Target))
		require.Equal(t, GateDropinContent([]Mount{testMount}, gateScript), tm.readFile(t, tm.dropinPath()))
		require.Empty(t, tm.warn.String())
	})

	t.Run("restarts a unit whose file changed", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, marker+"\noutdated content\n")
		tm.sys.EXPECT().DaemonReload(mock.Anything).Return(nil)
		tm.sys.EXPECT().Enable(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().Restart(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().ExecutorDaemonReload(mock.Anything).Return(nil)

		require.NoError(t, tm.Apply(ctx, []Mount{testMount}))

		require.Equal(t, UnitContent(testMount), tm.readFile(t, tm.unitPath(testUnit)))
	})

	t.Run("starts a unit whose file is unchanged", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, UnitContent(testMount))
		tm.sys.EXPECT().DaemonReload(mock.Anything).Return(nil)
		tm.sys.EXPECT().Enable(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().Start(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().ExecutorDaemonReload(mock.Anything).Return(nil)

		require.NoError(t, tm.Apply(ctx, []Mount{testMount}))
	})

	t.Run("removes stale units and the gate drop-in", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, marker+"\nmanaged, but no longer desired\n")
		tm.writeUnit(t, "var-mnt-foreign.mount", "not managed by selenite\n")
		require.NoError(t, os.MkdirAll(filepath.Dir(tm.dropinPath()), 0o755))           //nolint:gosec // scratch files in a test sandbox
		require.NoError(t, os.WriteFile(tm.dropinPath(), []byte("old drop-in"), 0o644)) //nolint:gosec // scratch files in a test sandbox
		tm.sys.EXPECT().Stop(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().Disable(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().DaemonReload(mock.Anything).Return(nil)
		tm.sys.EXPECT().ExecutorDaemonReload(mock.Anything).Return(nil)

		require.NoError(t, tm.Apply(ctx, nil))

		require.NoFileExists(t, tm.unitPath(testUnit))
		require.FileExists(t, tm.unitPath("var-mnt-foreign.mount"))
		require.NoFileExists(t, tm.dropinPath())
	})

	t.Run("rejects invalid mounts without touching the system", func(t *testing.T) {
		t.Parallel()
		// No mock expectations: any systemctl call fails the test.
		tm := newTestManager(t)

		err := tm.Apply(ctx, []Mount{{Type: "cifs", Source: "//nas/video", Target: "/var/mnt/video"}})
		require.ErrorContains(t, err, "unsupported type")
		require.NoFileExists(t, tm.unitPath(testUnit))
	})

	t.Run("tolerates mount and user reload failures", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.sys.EXPECT().DaemonReload(mock.Anything).Return(nil)
		tm.sys.EXPECT().Enable(mock.Anything, testUnit).Return(nil)
		tm.sys.EXPECT().Restart(mock.Anything, testUnit).Return(errors.New("storage server offline"))
		tm.sys.EXPECT().ExecutorDaemonReload(mock.Anything).Return(errors.New("no such user"))

		require.NoError(t, tm.Apply(ctx, []Mount{testMount}))

		require.Contains(t, tm.warn.String(), "restart "+testUnit+" failed (storage server offline?)")
		require.Contains(t, tm.warn.String(), "executor daemon-reload")
	})

	t.Run("enable failure is fatal", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.sys.EXPECT().DaemonReload(mock.Anything).Return(nil)
		tm.sys.EXPECT().Enable(mock.Anything, testUnit).Return(errors.New("boom"))

		err := tm.Apply(ctx, []Mount{testMount})
		require.ErrorContains(t, err, "enable "+testUnit)
	})
}

func TestManagerCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok when all managed mounts are active", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, UnitContent(testMount))
		tm.writeUnit(t, "var-mnt-foreign.mount", "not managed by selenite\n")
		tm.sys.EXPECT().IsActive(mock.Anything, testUnit).Return(true)

		require.NoError(t, tm.Check(ctx))
	})

	t.Run("reports inactive managed mounts", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, UnitContent(testMount))
		tm.writeUnit(t, "var-mnt-data.mount", marker+"\nmanaged\n")
		tm.sys.EXPECT().IsActive(mock.Anything, "var-mnt-data.mount").Return(true)
		tm.sys.EXPECT().IsActive(mock.Anything, testUnit).Return(false)

		err := tm.Check(ctx)
		require.ErrorContains(t, err, "inactive storage mounts: "+testUnit)
	})
}

func TestManagerRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("starts inactive mounts only", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, UnitContent(testMount))
		tm.writeUnit(t, "var-mnt-data.mount", marker+"\nmanaged\n")
		tm.sys.EXPECT().IsActive(mock.Anything, "var-mnt-data.mount").Return(true)
		tm.sys.EXPECT().IsActive(mock.Anything, testUnit).Return(false)
		tm.sys.EXPECT().Start(mock.Anything, testUnit).Return(nil)

		require.NoError(t, tm.Retry(ctx))
	})

	t.Run("tolerates start failures", func(t *testing.T) {
		t.Parallel()
		tm := newTestManager(t)
		tm.writeUnit(t, testUnit, UnitContent(testMount))
		tm.sys.EXPECT().IsActive(mock.Anything, testUnit).Return(false)
		tm.sys.EXPECT().Start(mock.Anything, testUnit).Return(errors.New("storage server offline"))

		require.NoError(t, tm.Retry(ctx))
	})
}
