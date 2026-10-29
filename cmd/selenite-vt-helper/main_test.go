// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		vt      int
		wantErr bool
	}{
		{name: "kmsg-redirect", args: []string{"kmsg-redirect", "1"}, vt: 1},
		{name: "max vt", args: []string{"kmsg-redirect", "63"}, vt: 63},
		{name: "no args", args: nil, wantErr: true},
		{name: "missing vt", args: []string{"kmsg-redirect"}, wantErr: true},
		{name: "extra args", args: []string{"kmsg-redirect", "1", "2"}, wantErr: true},
		{name: "unknown command", args: []string{"activate", "2"}, wantErr: true},
		{name: "not a number", args: []string{"kmsg-redirect", "tty1"}, wantErr: true},
		{name: "zero", args: []string{"kmsg-redirect", "0"}, wantErr: true},
		{name: "too big", args: []string{"kmsg-redirect", "64"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vt, err := parseArgs(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.vt, vt)
		})
	}
}
