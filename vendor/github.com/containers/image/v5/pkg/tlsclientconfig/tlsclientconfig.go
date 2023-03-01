package tlsclientconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

// SetupCertificates opens all .crt, .cert, and .key files in dir and appends / loads certs and key pairs as appropriate to tlsc
func SetupCertificates(dir string, tlsc *tls.Config) error {
	logrus.Debugf("Looking for TLS certificates and private keys in %s", dir)
	fs, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		if os.IsPermission(err) {
			logrus.Debugf("Skipping scan of %s due to permission error: %v", dir, err)
			return nil
		}
		return err
	}

	for _, f := range fs {
		fullPath := filepath.Join(dir, f.Name())
		if strings.HasSuffix(f.Name(), ".crt") {
			logrus.Debugf(" crt: %s", fullPath)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					// Dangling symbolic link?
					// Race with someone who deleted the
					// file after we read the directory's
					// list of contents?
					logrus.Warnf("error reading certificate %q: %v", fullPath, err)
					continue
				}
				return err
			}
			if tlsc.RootCAs == nil {
				systemPool, err := x509.SystemCertPool()
				if err != nil {
					return fmt.Errorf("unable to get system cert pool: %w", err)
				}
				tlsc.RootCAs = systemPool
			}
			tlsc.RootCAs.AppendCertsFromPEM(data)
		}
		if strings.HasSuffix(f.Name(), ".cert") {
			certName := f.Name()
			keyName := certName[:len(certName)-5] + ".key"
			logrus.Debugf(" cert: %s", fullPath)
			if !hasFile(fs, keyName) {
				return fmt.Errorf("missing key %s for client certificate %s. Note that CA certificates should use the extension .crt", keyName, certName)
			}
			cert, err := tls.LoadX509KeyPair(filepath.Join(dir, certName), filepath.Join(dir, keyName))
			if err != nil {
				return err
			}
			tlsc.Certificates = append(tlsc.Certificates, cert)
		}
		if strings.HasSuffix(f.Name(), ".key") {
			keyName := f.Name()
			certName := keyName[:len(keyName)-4] + ".cert"
			logrus.Debugf(" key: %s", fullPath)
			if !hasFile(fs, certName) {
				return fmt.Errorf("missing client certificate %s for key %s", certName, keyName)
			}
		}
	}
	return nil
}

func hasFile(files []os.DirEntry, name string) bool {
	return slices.ContainsFunc(files, func(f os.DirEntry) bool {
		return f.Name() == name
	})
}

// NewTransport Creates a default transport
func NewTransport() *http.Transport {
	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         direct.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        100,
	}
	return tr
}
