// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package compose

import (
	"fmt"
	"regexp"
)

var validName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid stack name %q: must match [a-z0-9][a-z0-9-]*", name)
	}
	return nil
}
