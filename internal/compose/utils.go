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
