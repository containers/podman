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
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"io"
	"time"
)

var keyUsageNames = map[x509.KeyUsage]string{
	x509.KeyUsageDigitalSignature:  "digitalSignature",
	x509.KeyUsageContentCommitment: "nonRepudiation",
	x509.KeyUsageKeyEncipherment:   "keyEncipherment",
	x509.KeyUsageDataEncipherment:  "dataEncipherment",
	x509.KeyUsageKeyAgreement:      "keyAgreement",
	x509.KeyUsageCertSign:          "keyCertSign",
	x509.KeyUsageCRLSign:           "cRLSign",
	x509.KeyUsageEncipherOnly:      "encipherOnly",
	x509.KeyUsageDecipherOnly:      "decipherOnly",
}

var extKeyUsageNames = map[x509.ExtKeyUsage]string{
	x509.ExtKeyUsageAny:             "any",
	x509.ExtKeyUsageServerAuth:      "serverAuth",
	x509.ExtKeyUsageClientAuth:      "clientAuth",
	x509.ExtKeyUsageCodeSigning:     "codeSigning",
	x509.ExtKeyUsageEmailProtection: "emailProtection",
	x509.ExtKeyUsageIPSECEndSystem:  "ipsecEndSystem",
	x509.ExtKeyUsageIPSECTunnel:     "ipsecTunnel",
	x509.ExtKeyUsageIPSECUser:       "ipsecUser",
	x509.ExtKeyUsageTimeStamping:    "timeStamping",
	x509.ExtKeyUsageOCSPSigning:     "OCSPSigning",

	x509.ExtKeyUsageMicrosoftServerGatedCrypto:     "msServerGatedCrypto",
	x509.ExtKeyUsageNetscapeServerGatedCrypto:      "nsServerGatedCrypto",
	x509.ExtKeyUsageMicrosoftCommercialCodeSigning: "msCodeCom",
	x509.ExtKeyUsageMicrosoftKernelCodeSigning:     "msKernCode",
}

var knownExtensions = []asn1.ObjectIdentifier{
	asn1.ObjectIdentifier{2, 5, 29, 14},               // oidExtensionSubjectKeyId
	asn1.ObjectIdentifier{2, 5, 29, 15},               // oidExtensionKeyUsage
	asn1.ObjectIdentifier{2, 5, 29, 37},               // oidExtensionExtendedKeyUsage
	asn1.ObjectIdentifier{2, 5, 29, 35},               // oidExtensionAuthorityKeyId
	asn1.ObjectIdentifier{2, 5, 29, 19},               // oidExtensionBasicConstraints
	asn1.ObjectIdentifier{2, 5, 29, 17},               // oidExtensionSubjectAltName
	asn1.ObjectIdentifier{2, 5, 29, 32},               // oidExtensionCertificatePolicies
	asn1.ObjectIdentifier{2, 5, 29, 30},               // oidExtensionNameConstraints
	asn1.ObjectIdentifier{2, 5, 29, 31},               // oidExtensionCRLDistributionPoints
	asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 1},  // oidExtensionAuthorityInfoAccess
	asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 1}, // oidAuthorityInfoAccessOcsp
	asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 2}, // oidAuthorityInfoAccessIssuers
}

// FprintCertificate formats a certificate for display
func FprintCertificate(w io.Writer, cert *x509.Certificate) {
	fmt.Fprintln(w, "Version:", cert.Version)
	if cert.SerialNumber.BitLen() > 63 {
		fmt.Fprintf(w, "Serial:  0x%x\n", cert.SerialNumber)
	} else {
		fmt.Fprintf(w, "Serial:  %d (0x%x)\n", cert.SerialNumber, cert.SerialNumber)
	}
	fmt.Fprintln(w, "Subject:", FormatSubject(cert))
	fmt.Fprintln(w, "Issuer: ", FormatIssuer(cert))
	fmt.Fprintln(w, "Valid:  ", cert.NotBefore)
	fmt.Fprintln(w, "Expires:", cert.NotAfter)
	fmt.Fprintln(w, "Period: ", subDate(cert.NotAfter, cert.NotBefore))
	switch k := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		n := fmt.Sprintf("%x", k.N)
		fmt.Fprintf(w, "Pub key: RSA bits=%d e=%d n=%s...%s\n", k.N.BitLen(), k.E, n[:8], n[len(n)-8:])
	case *ecdsa.PublicKey:
		p := k.Params()
		x := fmt.Sprintf("%x", k.X)
		y := fmt.Sprintf("%x", k.Y)
		fmt.Fprintf(w, "Public key: ECDSA bits=%d name=%s x=%s... y=...%s\n", p.BitSize, p.Name, x[:8], y[len(y)-8:])
	default:
		fmt.Fprintf(w, "Public key: %T\n", k)
	}
	fmt.Fprintln(w, "Sig alg:", cert.SignatureAlgorithm)
	fmt.Fprintln(w, "Extensions:")
	// subject alternate names
	printSAN(w, cert)
	// basic constraints
	if cert.BasicConstraintsValid {
		cons := fmt.Sprintf("isCA=%t", cert.IsCA)
		if cert.MaxPathLenZero {
			cons += " MaxPathLen=0"
		} else if cert.MaxPathLen > 0 {
			cons += fmt.Sprintf(" MaxPathLen=%d", cert.MaxPathLen)
		}
		fmt.Fprintln(w, "  Basic constraints: "+cons)
	}
	// Name constraints
	printNameConstraints(w, cert)
	// key usage
	usage := ""
	for n, name := range keyUsageNames {
		if cert.KeyUsage&n != 0 {
			usage += ", " + name
		}
	}
	if usage != "" {
		fmt.Fprintln(w, "  Key Usage:", usage[2:])
	}
	// extended key usage
	usage = ""
	for _, u := range cert.ExtKeyUsage {
		name := extKeyUsageNames[u]
		if name == "" {
			name = fmt.Sprintf("%d", u)
		}
		usage += ", " + name
	}
	for _, u := range cert.UnknownExtKeyUsage {
		usage += ", " + u.String()
	}
	if usage != "" {
		fmt.Fprintln(w, "  Extended key usage:", usage[2:])
	}
	// keyids
	if len(cert.SubjectKeyId) != 0 {
		fmt.Fprintf(w, "  Subject key ID: %x\n", cert.SubjectKeyId)
	}
	if len(cert.AuthorityKeyId) != 0 {
		fmt.Fprintf(w, "  Authority key ID: %x\n", cert.AuthorityKeyId)
	}
	// authority info
	if len(cert.OCSPServer) != 0 {
		fmt.Fprintln(w, "  OCSP Servers:")
		for _, s := range cert.OCSPServer {
			fmt.Fprintln(w, "   ", s)
		}
	}
	if len(cert.IssuingCertificateURL) != 0 {
		fmt.Fprintln(w, "  Issuing authority URLs:")
		for _, s := range cert.IssuingCertificateURL {
			fmt.Fprintln(w, "   ", s)
		}
	}
	// CRL
	if len(cert.CRLDistributionPoints) != 0 {
		fmt.Fprintln(w, "  CRL Distribution Points:")
		for _, s := range cert.CRLDistributionPoints {
			fmt.Fprintln(w, "   ", s)
		}
	}
	// Policy IDs
	if len(cert.PolicyIdentifiers) != 0 {
		fmt.Fprintln(w, "  Policy Identifiers:")
		for _, s := range cert.PolicyIdentifiers {
			fmt.Fprintln(w, "   ", s.String())
		}
	}
	// Other
	for _, ex := range cert.Extensions {
		if knownExtension(ex.Id) {
			continue
		}
		critical := ""
		if ex.Critical {
			critical = " (critical)"
		}
		fmt.Fprintf(w, "  Extension %s%s: %x\n", ex.Id, critical, ex.Value)
	}
}

func knownExtension(id asn1.ObjectIdentifier) bool {
	for _, known := range knownExtensions {
		if known.Equal(id) {
			return true
		}
	}
	return false
}

const (
	durYear  = 8766 * time.Hour
	yearSlop = 2 * 24 * time.Hour
)

// calculate duration using calendar dates
func subDate(end, start time.Time) string {
	approx := "~"
	dur := end.Sub(start)
	switch {
	case dur >= durYear-yearSlop:
		years := int((dur + yearSlop) / durYear)
		if start.AddDate(years, 0, 0).Equal(end) {
			approx = ""
		}
		if years > 1 {
			return fmt.Sprintf("%s%d years", approx, years)
		}
		return approx + "1 year"
	case dur >= 24*time.Hour:
		days := int(dur / (24 * time.Hour))
		if start.AddDate(0, 0, days).Equal(end) {
			approx = ""
		}
		if days > 1 {
			return fmt.Sprintf("%s%d days", approx, days)
		}
		return approx + "1 day"
	default:
		return dur.String()
	}
}

func printSAN(w io.Writer, cert *x509.Certificate) {
	if len(cert.DNSNames) != 0 || len(cert.EmailAddresses) != 0 || len(cert.IPAddresses) != 0 || len(cert.URIs) != 0 {
		fmt.Fprintln(w, "  Subject alternate names:")
		for _, s := range cert.DNSNames {
			fmt.Fprintln(w, "    dns:"+s)
		}
		for _, s := range cert.EmailAddresses {
			fmt.Fprintln(w, "    email:"+s)
		}
		for _, s := range cert.IPAddresses {
			fmt.Fprintln(w, "    ip:"+s.String())
		}
		for _, s := range cert.URIs {
			fmt.Fprintln(w, "    uri:"+s.String())
		}
	}
}

func printNameConstraints(w io.Writer, cert *x509.Certificate) {
	if len(cert.PermittedDNSDomains) != 0 || len(cert.ExcludedDNSDomains) != 0 || len(cert.PermittedIPRanges) != 0 || len(cert.ExcludedIPRanges) != 0 || len(cert.PermittedEmailAddresses) != 0 || len(cert.ExcludedEmailAddresses) != 0 || len(cert.PermittedURIDomains) != 0 || len(cert.ExcludedURIDomains) != 0 {
		fmt.Fprintln(w, "  Name constraints:")
		for _, s := range cert.PermittedDNSDomains {
			fmt.Fprintln(w, "     Permitted DNS domain:", s)
		}
		for _, s := range cert.ExcludedDNSDomains {
			fmt.Fprintln(w, "     Excluded DNS domain:", s)
		}
		for _, s := range cert.PermittedIPRanges {
			fmt.Fprintln(w, "     Permitted IP range:", s)
		}
		for _, s := range cert.ExcludedIPRanges {
			fmt.Fprintln(w, "     Excluded IP range:", s)
		}
		for _, s := range cert.PermittedEmailAddresses {
			fmt.Fprintln(w, "     Permitted Email Addresses:", s)
		}
		for _, s := range cert.ExcludedEmailAddresses {
			fmt.Fprintln(w, "     Excluded Email Addresses:", s)
		}
		for _, s := range cert.PermittedURIDomains {
			fmt.Fprintln(w, "     Permitted URI domain:", s)
		}
		for _, s := range cert.ExcludedURIDomains {
			fmt.Fprintln(w, "     Excluded URI domain:", s)
		}
	}
}
