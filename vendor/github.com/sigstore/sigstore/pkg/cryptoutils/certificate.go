//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cryptoutils

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	CertificatePEMType PEMType = "CERTIFICATE"
)

// MarshalCertificateToPEM converts the provided X509 certificate into PEM format
func MarshalCertificateToPEM(cert *x509.Certificate) ([]byte, error) {
	if cert == nil {
		return nil, errors.New("nil certificate provided")
	}
	return PEMEncode(CertificatePEMType, cert.Raw), nil
}

// MarshalCertificatesToPEM converts the provided X509 certificates into PEM format
func MarshalCertificatesToPEM(certs []*x509.Certificate) ([]byte, error) {
	buf := bytes.Buffer{}
	for _, cert := range certs {
		pemBytes, err := MarshalCertificateToPEM(cert)
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(pemBytes)
	}
	return buf.Bytes(), nil
}

// UnmarshalCertificatesFromPEM extracts one or more X509 certificates from the provided
// byte slice, which is assumed to be in PEM-encoded format.
func UnmarshalCertificatesFromPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	result := []*x509.Certificate{}
	remaining := pemBytes

	for len(remaining) > 0 {
		var certDer *pem.Block
		certDer, remaining = pem.Decode(remaining)

		if certDer == nil {
			return nil, errors.New("error during PEM decoding")
		}

		cert, err := x509.ParseCertificate(certDer.Bytes)
		if err != nil {
			return nil, err
		}
		result = append(result, cert)
	}
	return result, nil
}

// LoadCertificatesFromPEMFile extracts one or more X509 certificates from the provided
// io.Reader.
func LoadCertificatesFromPEM(pem io.Reader) ([]*x509.Certificate, error) {
	fileBytes, err := io.ReadAll(pem)
	if err != nil {
		return nil, err
	}
	return UnmarshalCertificatesFromPEM(fileBytes)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// CheckExpiration verifies that epoch is during the validity period of
// the certificate provided.
//
// It returns nil if issueTime < epoch < expirationTime, and error otherwise.
func CheckExpiration(cert *x509.Certificate, epoch time.Time) error {
	if cert == nil {
		return errors.New("certificate is nil")
	}
	if cert.NotAfter.Before(epoch) {
		return fmt.Errorf("certificate expiration time %s is before %s", formatTime(cert.NotAfter), formatTime(epoch))
	}
	if cert.NotBefore.After(epoch) {
		return fmt.Errorf("certificate issued time %s is before %s", formatTime(cert.NotBefore), formatTime(epoch))
	}
	return nil
}
