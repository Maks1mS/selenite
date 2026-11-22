// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package image

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
)

func newTestImageManager(t *testing.T, runner *mocks.MockRunner) *imageManager {
	t.Helper()
	return &imageManager{
		runner: runner,
	}
}

func TestLoad(t *testing.T) {
	t.Run("streams archive to helper", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"load"}, mock.Anything, mock.Anything).
			Return(nil).
			Once()

		var out bytes.Buffer
		err := newTestImageManager(t, r).Load(t.Context(), strings.NewReader("archive"), &out)
		assert.NoError(t, err)
	})

	t.Run("load error", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"load"}, mock.Anything, mock.Anything).
			Return(errors.New("boom")).
			Once()

		err := newTestImageManager(t, r).Load(t.Context(), strings.NewReader("archive"), io.Discard)
		assert.ErrorContains(t, err, "image load")
	})
}

func TestList(t *testing.T) {
	t.Run("lists images", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"list"}, mock.Anything).
			Return(nil).
			Once()

		var out bytes.Buffer
		err := newTestImageManager(t, r).List(t.Context(), &out)
		assert.NoError(t, err)
	})

	t.Run("list error", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"list"}, mock.Anything).
			Return(errors.New("boom")).
			Once()

		err := newTestImageManager(t, r).List(t.Context(), io.Discard)
		assert.ErrorContains(t, err, "image list")
	})
}

func TestRemove(t *testing.T) {
	t.Run("removes image by name", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"rm", "myapp:1.0"}, mock.Anything).
			Return(nil).
			Once()

		err := newTestImageManager(t, r).Remove(t.Context(), "myapp:1.0", io.Discard)
		assert.NoError(t, err)
	})

	t.Run("remove error", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(t.Context(), ExecutorBin, []string{"rm", "myapp:1.0"}, mock.Anything).
			Return(errors.New("boom")).
			Once()

		err := newTestImageManager(t, r).Remove(t.Context(), "myapp:1.0", io.Discard)
		assert.ErrorContains(t, err, "image remove")
	})
}

func TestPrune(t *testing.T) {
	r := mocks.NewMockRunner(t)
	r.EXPECT().
		Run(t.Context(), ExecutorBin, []string{"prune", "-f"}, mock.Anything).
		Return(nil).
		Once()

	var out bytes.Buffer
	err := newTestImageManager(t, r).Prune(t.Context(), &out)
	assert.NoError(t, err)
}

func TestNew(t *testing.T) {
	require.NotNil(t, New())
}
