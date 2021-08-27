//
// Copyright (c) SAS Institute Inc.
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
//

package x509tools

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	ArgCountry            string
	ArgOrganization       string
	ArgOrganizationalUnit string
	ArgLocality           string
	ArgProvince           string
	ArgCommonName         string
	ArgDNSNames           string
	ArgEmailNames         string
	ArgKeyUsage           string
	ArgExpireDays         uint
	ArgCertAuthority      bool
	ArgSerial             string
	ArgInteractive        bool
	ArgRSAPSS             bool
)

// Add flags associated with X509 requests to the given command
func AddRequestFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&ArgCountry, "countryName", "", "Subject name")
	cmd.Flags().StringVar(&ArgProvince, "stateOrProvinceName", "", "Subject name")
	cmd.Flags().StringVar(&ArgLocality, "localityName", "", "Subject name")
	cmd.Flags().StringVar(&ArgOrganization, "organizationName", "", "Subject name")
	cmd.Flags().StringVar(&ArgOrganizationalUnit, "organizationalUnitName", "", "Subject name")
	cmd.Flags().StringVarP(&ArgCommonName, "commonName", "n", "", "Subject commonName")
	cmd.Flags().StringVar(&ArgDNSNames, "alternate-dns", "", "DNS subject alternate name (comma or space separated)")
	cmd.Flags().StringVar(&ArgEmailNames, "alternate-email", "", "Email subject alternate name (comma or space separated)")
	cmd.Flags().BoolVarP(&ArgInteractive, "interactive", "i", false, "Prompt before signing certificate")
	cmd.Flags().BoolVar(&ArgRSAPSS, "rsa-pss", false, "Use RSA-PSS signature")
}

// Add flags associated with X509 certificate creation to the given command
func AddCertFlags(cmd *cobra.Command) {
	AddRequestFlags(cmd)
	cmd.Flags().BoolVar(&ArgCertAuthority, "cert-authority", false, "If this certificate is an authority")
	cmd.Flags().StringVarP(&ArgKeyUsage, "key-usage", "U", "", "Key usage, one of: serverAuth clientAuth codeSigning emailProtection keyCertSign")
	cmd.Flags().UintVarP(&ArgExpireDays, "expire-days", "e", 36523, "Number of days before certificate expires")
	cmd.Flags().StringVar(&ArgSerial, "serial", "", "Set the serial number of the certificate. Random if not specified.")
}

// Split a space- and/or comma-seperated string
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.Replace(s, ",", " ", -1)
	pieces := strings.Split(s, " ")
	ret := make([]string, 0, len(pieces))
	for _, p := range pieces {
		p = strings.Trim(p, " ")
		if p != "" {
			ret = append(ret, p)
		}
	}
	return ret
}

// Build a subject name from command-line arguments
func subjName() (name pkix.Name) {
	if ArgCountry != "" {
		name.Country = []string{ArgCountry}
	}
	if ArgProvince != "" {
		name.Province = []string{ArgProvince}
	}
	if ArgLocality != "" {
		name.Locality = []string{ArgLocality}
	}
	if ArgOrganization != "" {
		name.Organization = []string{ArgOrganization}
	}
	if ArgOrganizationalUnit != "" {
		name.OrganizationalUnit = []string{ArgOrganizationalUnit}
	}
	name.CommonName = ArgCommonName
	return
}

// Set both basic and extended key usage
func setUsage(template *x509.Certificate) error {
	usage := x509.KeyUsageDigitalSignature
	var extended []x509.ExtKeyUsage
	switch strings.ToLower(ArgKeyUsage) {
	case "serverauth":
		usage |= x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement
		extended = append(extended, x509.ExtKeyUsageServerAuth)
	case "clientauth":
		usage |= x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement
		extended = append(extended, x509.ExtKeyUsageClientAuth)
	case "codesigning":
		extended = append(extended, x509.ExtKeyUsageCodeSigning)
	case "emailprotection":
		usage |= x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement
		extended = append(extended, x509.ExtKeyUsageEmailProtection)
	case "keycertsign":
		usage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	case "":
		return nil
	default:
		return errors.New("invalid key-usage")
	}
	template.KeyUsage = usage
	template.ExtKeyUsage = extended
	return nil
}

func fillCertFields(template *x509.Certificate, subjectPub, issuerPub crypto.PublicKey) error {
	if ArgSerial != "" {
		serial, ok := new(big.Int).SetString(ArgSerial, 0)
		if !ok {
			return errors.New("invalid serial number, must be decimal or hexadecimal format")
		}
		template.SerialNumber = serial
	} else if template.SerialNumber == nil {
		template.SerialNumber = MakeSerial()
		if template.SerialNumber == nil {
			return errors.New("Failed to generate a serial number")
		}
	}
	if ArgCommonName != "" {
		template.Subject = subjName()
	}
	if ArgDNSNames != "" {
		template.DNSNames = splitAndTrim(ArgDNSNames)
	}
	if ArgEmailNames != "" {
		template.EmailAddresses = splitAndTrim(ArgEmailNames)
	}
	template.SignatureAlgorithm = X509SignatureAlgorithm(issuerPub)
	template.NotBefore = time.Now().Add(time.Hour * -24)
	template.NotAfter = time.Now().Add(time.Hour * 24 * time.Duration(ArgExpireDays))
	template.IsCA = ArgCertAuthority
	template.BasicConstraintsValid = true
	ski, err := SubjectKeyID(subjectPub)
	if err != nil {
		return err
	}
	template.SubjectKeyId = ski
	return setUsage(template)
}

func toPemString(der []byte, pemType string) string {
	block := &pem.Block{Type: pemType, Bytes: der}
	return string(pem.EncodeToMemory(block))
}

// Make a X509 certificate request using command-line arguments and return the
// PEM string
func MakeRequest(rand io.Reader, key crypto.Signer) (string, error) {
	var template x509.CertificateRequest
	template.Subject = subjName()
	template.DNSNames = splitAndTrim(ArgDNSNames)
	template.EmailAddresses = splitAndTrim(ArgEmailNames)
	template.SignatureAlgorithm = X509SignatureAlgorithm(key.Public())
	csr, err := x509.CreateCertificateRequest(rand, &template, key)
	if err != nil {
		return "", err
	}
	return toPemString(csr, "CERTIFICATE REQUEST"), nil
}

// Make a self-signed X509 certificate using command-line arguments and return
// the PEM string
func MakeCertificate(rand io.Reader, key crypto.Signer) (string, error) {
	var template x509.Certificate
	if err := fillCertFields(&template, key.Public(), key.Public()); err != nil {
		return "", err
	}
	template.Issuer = template.Subject
	cert, err := confirmAndCreate(&template, &template, key.Public(), key)
	if err != nil {
		return "", err
	}
	return toPemString(cert, "CERTIFICATE"), nil
}

// SignCSR takes a PKCS#10 signing request in PEM or DER format as input and
// produces a signed certificate in PEM format. Any command-line flags set will
// override the CSR contents.
func SignCSR(csrBytes []byte, rand io.Reader, key crypto.Signer, cacert *x509.Certificate, copyExtensions bool) (string, error) {
	// parse and validate CSR
	csrBytes, err := parseMaybePEM(csrBytes, "CERTIFICATE REQUEST")
	if err != nil {
		return "", err
	}
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return "", fmt.Errorf("parsing CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return "", fmt.Errorf("validating CSR: %w", err)
	}
	// update fields
	template := &x509.Certificate{Subject: csr.Subject}
	if copyExtensions {
		template.ExtraExtensions = csr.Extensions
		template.DNSNames = csr.DNSNames
		template.EmailAddresses = csr.EmailAddresses
		template.IPAddresses = csr.IPAddresses
		template.URIs = csr.URIs
	}
	if err := fillCertFields(template, csr.PublicKey, key.Public()); err != nil {
		return "", err
	}
	certDer, err := confirmAndCreate(template, cacert, csr.PublicKey, key)
	if err != nil {
		return "", err
	}
	return toPemString(certDer, "CERTIFICATE"), nil
}

// CrossSign takes a certificate as input and re-signs it using the given key.
// Any command-line flags set will override the CSR contents.
func CrossSign(certBytes []byte, rand io.Reader, key crypto.Signer, cacert *x509.Certificate) (string, error) {
	certBytes, err := parseMaybePEM(certBytes, "CERTIFICATE")
	if err != nil {
		return "", err
	}
	template, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return "", fmt.Errorf("parsing certificate: %w", err)
	}
	if err := fillCertFields(template, template.PublicKey, key.Public()); err != nil {
		return "", err
	}
	newCert, err := confirmAndCreate(template, cacert, template.PublicKey, key)
	if err != nil {
		return "", err
	}
	return toPemString(newCert, "CERTIFICATE"), nil
}

type fakeSigner struct{ pub crypto.PublicKey }

func (f fakeSigner) Public() crypto.PublicKey {
	return f.pub
}

func (f fakeSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, nil
}

func confirmAndCreate(template, parent *x509.Certificate, pub crypto.PublicKey, priv crypto.PrivateKey) ([]byte, error) {
	if ArgInteractive {
		// call CreateCertificate with a fake signer to get what the final cert will look like
		pub := priv.(crypto.Signer).Public()
		der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, fakeSigner{pub})
		if err != nil {
			return nil, err
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(os.Stderr, "Signing certificate:")
		fmt.Fprintln(os.Stderr)
		FprintCertificate(os.Stderr, cert)
		fmt.Fprintln(os.Stderr)
		if !promptYN("Sign this cert? [Y/n] ") {
			fmt.Fprintln(os.Stderr, "operation canceled")
			os.Exit(2)
		}
	}
	return x509.CreateCertificate(rand.Reader, template, parent, pub, priv)
}

func promptYN(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	if !terminal.IsTerminal(0) {
		fmt.Fprintln(os.Stderr, "input is not a terminal, assuming true")
		return true
	}
	state, err := terminal.MakeRaw(0)
	if err == nil {
		defer fmt.Fprintln(os.Stderr)
		defer terminal.Restore(0, state)
	}
	var d [1]byte
	if _, err := os.Stdin.Read(d[:]); err != nil {
		return false
	}
	if d[0] == 'Y' || d[0] == 'y' {
		return true
	}
	return false
}

func parseMaybePEM(blob []byte, pemType string) ([]byte, error) {
	if bytes.Contains(blob, []byte("-----BEGIN")) {
		for {
			var block *pem.Block
			block, blob = pem.Decode(blob)
			if block == nil {
				break
			} else if block.Type == pemType {
				return block.Bytes, nil
			}
		}
	} else if len(blob) > 0 && blob[0] == 0x30 {
		return blob, nil
	}
	return nil, fmt.Errorf("expected a %s in PEM or DER format", strings.ToLower(pemType))
}
