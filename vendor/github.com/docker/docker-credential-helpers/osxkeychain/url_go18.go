//+build go1.8

package osxkeychain

import "net/url"

func getHostname(u *url.URL) string {
	return u.Hostname()
}

func getPort(u *url.URL) string {
	return u.Port()
}
