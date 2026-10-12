// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package volume

import (
	"bytes"
	"errors"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
)

func newTestVolumeManager(t *testing.T, runner *mocks.MockRunner) *volumeManager {
	t.Helper()
	return &volumeManager{
		runner: runner,
	}
}

func zstdPayload(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	_, err = enc.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, enc.Close())
	return &buf
}

func TestImport(t *testing.T) {
	t.Run("creates volume before import", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		createCall := r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"create", "--ignore", "test"}).
			Return(nil).
			Once()
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"import", "test", "-"}, mock.Anything).
			Return(nil).
			Once().
			NotBefore(createCall)

		err := newTestVolumeManager(t, r).Import(t.Context(), "test", zstdPayload(t))
		assert.NoError(t, err)
	})

	t.Run("create error", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"create", "--ignore", "test"}).
			Return(errors.New("boom")).
			Once()

		err := newTestVolumeManager(t, r).Import(t.Context(), "test", zstdPayload(t))
		assert.ErrorContains(t, err, "volume create")
	})
}
