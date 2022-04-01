package containers

import (
	"context"
	"os"
	"time"

	sig "github.com/containers/podman/v4/pkg/signal"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/windows"
)

func makeRawTerm(stdin *os.File) (*terminal.State, error) {
	state, err := terminal.MakeRaw(int(stdin.Fd()))
	if err != nil {
		return nil, err
	}

	// Attempt VT if supported (recent versions of Windows 10+)
	var raw uint32
	handle := windows.Handle(stdin.Fd())
	if err := windows.GetConsoleMode(handle, &raw); err != nil {
		return nil, err
	}

	tryVT := raw | windows.ENABLE_VIRTUAL_TERMINAL_INPUT

	if err := windows.SetConsoleMode(handle, tryVT); err != nil {
		if err := windows.SetConsoleMode(handle, raw); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func notifyWinChange(ctx context.Context, winChange chan os.Signal, stdin *os.File, stdout *os.File) {
	// Simulate WINCH with polling
	go func() {
		var lastW int
		var lastH int

		d := time.Millisecond * 250
		timer := time.NewTimer(d)
		defer timer.Stop()
		for ; ; timer.Reset(d) {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				break
			}

			w, h, err := terminal.GetSize(int(stdout.Fd()))
			if err != nil {
				continue
			}
			if w != lastW || h != lastH {
				winChange <- sig.SIGWINCH
				lastW, lastH = w, h
			}
		}
	}()

}

func getTermSize(stdin *os.File, stdout *os.File) (width, height int, err error) {
	return terminal.GetSize(int(stdout.Fd()))
}
