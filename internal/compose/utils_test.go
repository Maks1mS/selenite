package compose

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateName(t *testing.T) {
	valid := []string{"mystack", "my-stack", "stack1", "a", "0stack", "abc-def-ghi"}
	for _, name := range valid {
		require.NoError(t, validateName(name), "validateName(%q) should be valid", name)
	}

	invalid := []string{"", "../evil", "My-Stack", "stack name", "-start", "Stack", "stack-", "stack--two"}
	for _, name := range invalid {
		require.Error(t, validateName(name), "validateName(%q) should be invalid", name)
	}
}
