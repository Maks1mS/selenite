package cmdutils

import (
	"fmt"
	"os"
)

func Fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
