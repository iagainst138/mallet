package term

import (
	"fmt"
	"os"
	"time"
)

const (
	writeInterval = time.Millisecond * 300

	previousLine = "\x1b[1F"
	clearLine    = "\r\x1b[2K"
)

var (
	stdErrIsTerminal bool
	stdOutIsTerminal bool

	lastWrite time.Time
)

func Fprintf(f *os.File, s string, args ...any) {
	shouldWrite := false

	if time.Since(lastWrite) < writeInterval {
		return
	}

	switch f {
	case os.Stderr:
		shouldWrite = stdErrIsTerminal
	case os.Stdout:
		shouldWrite = stdOutIsTerminal
	}

	if shouldWrite {
		update := fmt.Sprintf(s, args...)
		fmt.Fprintf(f, "%s%s%s\n", previousLine, clearLine, update)
		lastWrite = time.Now()
	}
}

func Flush() {
	if stdErrIsTerminal {
		fmt.Fprint(os.Stderr, previousLine)
		fmt.Fprint(os.Stderr, clearLine)
		fmt.Fprint(os.Stderr, "\r")
	}

	if stdOutIsTerminal {
		fmt.Fprint(os.Stdout, previousLine)
		fmt.Fprint(os.Stdout, clearLine)
		fmt.Fprint(os.Stdout, "\r")
	}
}

func init() {
	o, err := os.Stdout.Stat()
	if err == nil {
		if (o.Mode() & os.ModeCharDevice) == os.ModeCharDevice {
			stdOutIsTerminal = true
		}
	}

	e, err := os.Stderr.Stat()
	if err == nil {
		if (e.Mode() & os.ModeCharDevice) == os.ModeCharDevice {
			stdErrIsTerminal = true
		}
	}
}
