// Package out provides best-effort writes for CLI output (stdout/stderr).
// Errors from the writer are ignored; callers use real error handling for I/O that must succeed.
package out

import (
	"fmt"
	"io"
)

func Fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func Fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}
