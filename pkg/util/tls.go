package util

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func ReadCertBundle(path string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	caPEM, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cert bundle %s: %w", path, err)
	}
	for ix := 0; len(caPEM) != 0; ix++ {
		var caDER *pem.Block
		caDER, caPEM = pem.Decode(caPEM)
		if caDER == nil || caDER.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("non-certificate type `%s` PEM data found in cert bundle %s", caDER.Type, path)
		}
		caCert, err := x509.ParseCertificate(caDER.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing cert bundle at index %d: %w", ix, err)
		}
		pool.AddCert(caCert)
	}
	return pool, nil
}
