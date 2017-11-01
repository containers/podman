// +build windows plan9 solaris

package goterm

func getWinsize() (*winsize, error) {
	ws := new(winsize)

	ws.Col = 80
	ws.Row = 24

	return ws, nil
}
