// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"altlinux.space/alt-atomic/selenite/internal/dashboard"
)

func main() {
	m := dashboard.New()
	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
