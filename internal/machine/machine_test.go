package machine

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil/mocks"
	config "altlinux.space/alt-atomic-lab/selenite/internal/machine/config/v1"
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
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: 1024
`

		err := m.ApplyConfig(context.Background(), strings.NewReader(cfg), io.Discard)
		require.NoError(t, err)
	})
}
