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

//go:build e2e
// +build e2e

package x509

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"testing"

	"github.com/sigstore/rekor/pkg/util"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// Generated with:
// openssl ecparam -genkey -name prime256v1 > ec_private.pem
// openssl pkcs8 -topk8 -in ec_private.pem  -nocrypt
const ECDSAPriv = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgmrLtCpBdXgXLUr7o
nSUPfo3oXMjmvuwTOjpTulIBKlKhRANCAATH6KSpTFe6uXFmW1qNEFXaO7fWPfZt
pPZrHZ1cFykidZoURKoYXfkohJ+U/USYy8Sd8b4DMd5xDRZCnlDM0h37
-----END PRIVATE KEY-----`

// Extracted from above with:
// openssl ec -in ec_private.pem -pubout
const ECDSAPub = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEx+ikqUxXurlxZltajRBV2ju31j32
baT2ax2dXBcpInWaFESqGF35KISflP1EmMvEnfG+AzHecQ0WQp5QzNId+w==
-----END PUBLIC KEY-----`

// Generated with:
// openssl req -newkey rsa:2048 -nodes -keyout test.key -x509 -out test.crt
const RSACert = `-----BEGIN CERTIFICATE-----
MIIDOjCCAiKgAwIBAgIUEP925shVBKERFCsymdSqESLZFyMwDQYJKoZIhvcNAQEL
BQAwHzEdMBsGCSqGSIb3DQEJARYOdGVzdEByZWtvci5kZXYwHhcNMjEwNDIxMjAy
ODAzWhcNMjEwNTIxMjAyODAzWjAfMR0wGwYJKoZIhvcNAQkBFg50ZXN0QHJla29y
LmRldjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAN8KiP08rFIik4GN
W8/sHSXxDopeDBLEQEihsyXXWesfYW/q59lFaCZrsTetlyNEzKDJ+JrpIHwoGOo4
EwefFfvy2nkgPFs9aeIDsYZNZnIGxeB8sUfsZUYGHx+Ikm18vhM//GYzNjjuvHyq
+CWRAOS12ZISa99iah/lIhcP8IEj1gPGldAH0QFx3XpCePAdQocSU6ziVkj054/x
NJXy1bKySrVw7gvE9LxZlVO9urSOnzg7BBOla0mob8NRDVB8yN+LG365q4IMDzuI
jAEL6sLtoJ9pcemo1rIfNOhSLYlzfg7oszJ8eCjASNCCcp6EKVjhW7LRoldC8oGZ
EOrKM78CAwEAAaNuMGwwHQYDVR0OBBYEFGjs8EHKT3x1itwwptJLuQQg/hQcMB8G
A1UdIwQYMBaAFGjs8EHKT3x1itwwptJLuQQg/hQcMA8GA1UdEwEB/wQFMAMBAf8w
GQYDVR0RBBIwEIEOdGVzdEByZWtvci5kZXYwDQYJKoZIhvcNAQELBQADggEBAAHE
bYuePN3XpM7pHoCz6g4uTHu0VrezqJyK1ohysgWJmSJzzazUeISXk0xWnHPk1Zxi
kzoEuysI8b0P7yodMA8e16zbIOL6QbGe3lNXYqRIg+bl+4OPFGVMX8xHNZmeh0kD
vX1JVS+y9uyo4/z/pm0JhaSCn85ft/Y5uXMQYn1wFR5DAcJH+iWjNX4fipGxGRE9
Cy0DjFnYJ3SRY4HPQ0oUSQmyhrwe2DiYzeqtbL2KJBXPcFQKWhkf/fupdYFljvcH
d9NNfRb0p2oFGG/J0ROg9pEcP1/aZP5k8P2pRdt3y7h1MAtmg2bgEdugZgXwAUmM
BmU8k2FeTuqV15piPCE=
-----END CERTIFICATE-----`

const RSAKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDfCoj9PKxSIpOB
jVvP7B0l8Q6KXgwSxEBIobMl11nrH2Fv6ufZRWgma7E3rZcjRMygyfia6SB8KBjq
OBMHnxX78tp5IDxbPWniA7GGTWZyBsXgfLFH7GVGBh8fiJJtfL4TP/xmMzY47rx8
qvglkQDktdmSEmvfYmof5SIXD/CBI9YDxpXQB9EBcd16QnjwHUKHElOs4lZI9OeP
8TSV8tWyskq1cO4LxPS8WZVTvbq0jp84OwQTpWtJqG/DUQ1QfMjfixt+uauCDA87
iIwBC+rC7aCfaXHpqNayHzToUi2Jc34O6LMyfHgowEjQgnKehClY4Vuy0aJXQvKB
mRDqyjO/AgMBAAECggEBAIHOAs3Gis8+WjRSjXVjh882DG1QsJwXZQYgPT+vpiAl
YjKdNpOHRkbd9ARgXY5kEuccxDd7p7E6MM3XFpQf7M51ltpZfWboRgAIgD+WOiHw
eSbdytr95C6tj11twTJBH+naGk1sTokxv7aaVdKfIjL49oeBexBFmVe4pW9gkmrE
1z1y1a0RohqbZ0kprYPWjz5UhsNqbCzgkdDqS7IrcOwVg6zvKYFjHnqIHqaJXVif
FgIfoNt7tz+12FTHI+6OkKoN3YCJueaxneBhITXm6RLOpQWa9qhdUPbkJ9vQNfph
Qqke4faaxKY9UDma+GpEHR016AWufZp92pd9wQkDn0kCgYEA7w/ZizAkefHoZhZ8
Isn/fYu4fdtUaVgrnGUVZobiGxWrHRU9ikbAwR7UwbgRSfppGiJdAMq1lyH2irmb
4OHU64rjuYSlIqUWHLQHWmqUbLUvlDojH/vdmH/Zn0AbrLZaimC5UCjK3Eb7sAMq
G0tGeDX2JraQvx7KrbC6peTaaaMCgYEA7tgZBiRCQJ7+mNu+gX9x6OXtjsDCh516
vToRLkxWc7LAbC9LKsuEHl4e3vy1PY/nyuv12Ng2dBq4WDXozAmVgz0ok7rRlIFp
w8Yj8o/9KuGZkD/7tw/pLsVc9Q3Wf0ACrnAAh7+3dAvn3yg+WHwXzqWIbrseDPt9
ILCfUoNDpzUCgYAKFCX8y0PObFd67lm/cbq2xUw66iNN6ay1BEH5t5gSwkAbksis
ar03pyAbJrJ75vXFZ0t6fBFZ1NG7GYYr3fmHEKz3JlN7+W/MN/7TXgjx6FWgLy9J
6ul1w3YeU6qXBn0ctmU5ru6WiNuVmRyOWAcZjFTbXvkNRbQPzJKh6dsXdwKBgA1D
FIihxMf/zBVCxl48bF/JPJqbm3GaTfFp4wBWHsrH1yVqrtrOeCSTh1VMZOfpMK60
0W7b+pIR1cCYJbgGpDWoVLN3QSHk2bGUM/TJB/60jilTVC/DA2ikbtfwj8N7E2sK
Lw1amN4ptxNOEcAqC8xepqe3XiDMahNBm2cigMQtAoGBAKwrXvss2BKz+/6poJQU
A0c7jhMN8M9Y5S2Ockw07lrQeAgfu4q+/8ztm0NeHJbk01IJvJY5Nt7bSgwgNVlo
j7vR2BMAc9U73Ju9aeTl/L6GqmZyA+Ojhl5gA5DPZYqNiqi93ydgRaI6n4+o3dI7
5wnr40AmbuKCDvMOvN7nMybL
-----END PRIVATE KEY-----`

// Extracted from the certificate using:
// openssl x509 -pubkey -noout -in test.crt
const PubKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3wqI/TysUiKTgY1bz+wd
JfEOil4MEsRASKGzJddZ6x9hb+rn2UVoJmuxN62XI0TMoMn4mukgfCgY6jgTB58V
+/LaeSA8Wz1p4gOxhk1mcgbF4HyxR+xlRgYfH4iSbXy+Ez/8ZjM2OO68fKr4JZEA
5LXZkhJr32JqH+UiFw/wgSPWA8aV0AfRAXHdekJ48B1ChxJTrOJWSPTnj/E0lfLV
srJKtXDuC8T0vFmVU726tI6fODsEE6VrSahvw1ENUHzI34sbfrmrggwPO4iMAQvq
wu2gn2lx6ajWsh806FItiXN+DuizMnx4KMBI0IJynoQpWOFbstGiV0LygZkQ6soz
vwIDAQAB
-----END PUBLIC KEY-----`

var (
	CertPrivateKey *rsa.PrivateKey
	Certificate    *x509.Certificate
)

func init() {
	p, _ := pem.Decode([]byte(RSAKey))
	priv, err := x509.ParsePKCS8PrivateKey(p.Bytes)
	if err != nil {
		panic(err)
	}
	cpk, ok := priv.(*rsa.PrivateKey)
	if !ok {
		panic("unsuccessful conversion")
	}
	CertPrivateKey = cpk

	p, _ = pem.Decode([]byte(RSACert))
	Certificate, err = x509.ParseCertificate(p.Bytes)
	if err != nil {
		panic(err)
	}
}

func SignX509Cert(b []byte) ([]byte, error) {
	dgst := sha256.Sum256(b)
	signature, err := CertPrivateKey.Sign(rand.Reader, dgst[:], crypto.SHA256)
	return signature, err
}

// CreatedX509SignedArtifact gets the test dir setup correctly with some random artifacts and keys.
func CreatedX509SignedArtifact(t *testing.T, artifactPath, sigPath string) {
	t.Helper()
	artifact := util.CreateArtifact(t, artifactPath)

	// Sign it with our key and write that to a file
	signature, err := SignX509Cert([]byte(artifact))
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(sigPath, []byte(signature), 0644); err != nil {
		t.Fatal(err)
	}
}

type Verifier struct {
	S signature.Signer
	v signature.Verifier
}

func (v *Verifier) KeyID() (string, error) {
	return "", nil
}

func (v *Verifier) Public() crypto.PublicKey {
	return v.v.PublicKey
}

func (v *Verifier) Sign(_ context.Context, data []byte) (sig []byte, err error) {
	if v.S == nil {
		return nil, errors.New("nil signer")
	}
	sig, err = v.S.SignMessage(bytes.NewReader(data), options.WithCryptoSignerOpts(crypto.SHA256))
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func (v *Verifier) Verify(_ context.Context, data, sig []byte) error {
	if v.v == nil {
		return errors.New("nil Verifier")
	}
	return v.v.VerifySignature(bytes.NewReader(sig), bytes.NewReader(data))
}
