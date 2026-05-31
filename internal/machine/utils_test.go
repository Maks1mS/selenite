// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package machine

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
	"altlinux.space/alt-atomic-lab/selenite/internal/executil/mocks"
)

func TestPodmanVersion(t *testing.T) {
	fakeOutput := "podman-compose version 1.2.3\npodman version 4.5.6"

	tests := []struct {
		name     string
		output   string
		wantErr  bool
		expected PodmanVersion
	}{
		{
			name:   "valid output",
			output: fakeOutput,
			expected: PodmanVersion{
				Version:        "4.5.6",
				ComposeVersion: "1.2.3",
			},
		},
		{
			name:   "empty output",
			output: "",
			expected: PodmanVersion{
				Version:        "N/A",
				ComposeVersion: "N/A",
			},
		},
		{
			name:   "malformed output",
			output: "some random text",
			expected: PodmanVersion{
				Version:        "N/A",
				ComposeVersion: "N/A",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mocks.NewMockRunner(t)

			r.EXPECT().
				Run(
					context.Background(),
					"podman-compose",
					[]string{"--version"},
					mock.MatchedBy(func(opts []executil.Option) bool {
						cmd := &exec.Cmd{}
						for _, o := range opts {
							o(cmd)
						}
						if cmd.Stdout != nil {
							_, _ = cmd.Stdout.Write([]byte(tt.output))
						}
						return true
					}),
				).
				Return(nil)

			result := podmanVersion(context.Background(), r)
			assert.Equal(t, tt.expected, result)
		})
	}
}
