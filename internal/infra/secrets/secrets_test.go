// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package secrets

import (
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
	"altlinux.space/alt-atomic-lab/selenite/internal/executil/mocks"
)

func newTestSecretManager(t *testing.T, runner *mocks.MockRunner) *secretsManager {
	t.Helper()
	return &secretsManager{
		runner: runner,
	}
}

func TestCreateConfig(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		reader := strings.NewReader("Super secret string")
		r.EXPECT().
			Run(
				t.Context(),
				"/usr/libexec/selenite-controller/podman-secret-helper",
				[]string{"create", "test", "-"},
				mock.MatchedBy(func(opts []executil.Option) bool {
					cmd := &exec.Cmd{}
					for _, o := range opts {
						o(cmd)
					}
					return cmd.Stdin == reader
				}),
			).
			Return(nil)

		s := newTestSecretManager(t, r)

		err := s.Create(t.Context(), "test", reader, io.Discard)
		assert.NoError(t, err)
	})
}
