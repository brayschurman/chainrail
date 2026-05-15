package term

import (
	"io"
	"os"
)

// IsTTY reports whether w is connected to a character device (typically a
// terminal). With stdlib-only constraints this approximation cannot
// distinguish a real terminal from other character devices like /dev/null —
// that distinction would need a proper ioctl via golang.org/x/term. For our
// purpose (auto-plain output when piped to a non-terminal), the approximation
// is sufficient: pipes, files, and buffers — the only writers callers
// actually wire to non-interactive contexts — all return false correctly.
func IsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
