//
// Copyright 2022 The Sigstore Authors.
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

package util

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/openpgp"

	"github.com/sigstore/rekor/pkg/generated/models"
)

var (
	cli    = "rekor-cli"
	server = "rekor-server"
	keys   openpgp.EntityList
)

type GetOut struct {
	Attestation     string
	AttestationType string
	Body            interface{}
	LogIndex        int
	IntegratedTime  int64
}

// This was generated with gpg --gen-key, and all defaults.
// The email is "test@rekor.dev", and the name is Rekor Test.
// It should only be used for test purposes.

var secretKey = `-----BEGIN PGP PRIVATE KEY BLOCK-----

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
=CwjQ
-----END PGP PRIVATE KEY BLOCK-----`
var PrivateKey = `-----BEGIN PGP PRIVATE KEY BLOCK-----

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
=CwjQ
-----END PGP PRIVATE KEY BLOCK-----`

var PubKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
WR6Fv7ySHTl7cQ13jGmSR1N8hg==
=Fen+
-----END PGP PUBLIC KEY BLOCK-----`

func init() {
	p := os.Getenv("REKORTMPDIR")
	if p != "" {
		cli = path.Join(p, cli)
		server = path.Join(p, server)
	}
	var err error
	keys, err = openpgp.ReadArmoredKeyRing(strings.NewReader(secretKey))
	if err != nil {
		panic(err)
	}
}

func OutputContains(t *testing.T, output, sub string) {
	t.Helper()
	if !strings.Contains(output, sub) {
		t.Errorf("Expected [%s] in response, got %s", sub, output)
	}
}

func Run(t *testing.T, stdin, cmd string, arg ...string) string {
	t.Helper()
	arg = append([]string{coverageFlag()}, arg...)
	c := exec.Command(cmd, arg...)
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	if os.Getenv("REKORTMPDIR") != "" {
		// ensure that we use a clean state.json file for each Run
		c.Env = append(c.Env, "HOME="+os.Getenv("REKORTMPDIR"))
	}
	b, err := c.CombinedOutput()
	if err != nil {
		t.Log(string(b))
		t.Fatal(err)
	}
	return stripCoverageOutput(string(b))
}

func RunCli(t *testing.T, arg ...string) string {
	t.Helper()
	arg = append(arg, rekorServerFlag())
	// use a blank config file to ensure no collision
	if os.Getenv("REKORTMPDIR") != "" {
		arg = append(arg, "--config="+os.Getenv("REKORTMPDIR")+".rekor.yaml")
	}
	return Run(t, "", cli, arg...)
}

func RunCliErr(t *testing.T, arg ...string) string {
	t.Helper()
	arg = append([]string{coverageFlag()}, arg...)
	arg = append(arg, rekorServerFlag())
	// use a blank config file to ensure no collision
	if os.Getenv("REKORTMPDIR") != "" {
		arg = append(arg, "--config="+os.Getenv("REKORTMPDIR")+".rekor.yaml")
	}
	cmd := exec.Command(cli, arg...)
	b, err := cmd.CombinedOutput()
	if err == nil {
		t.Log(string(b))
		t.Fatalf("expected error, got %s", string(b))
	}
	return stripCoverageOutput(string(b))
}

func rekorServerFlag() string {
	return fmt.Sprintf("--rekor_server=%s", rekorServer())
}

func rekorServer() string {
	if s := os.Getenv("REKOR_SERVER"); s != "" {
		return s
	}
	return "http://localhost:3000"
}

func coverageFlag() string {
	return "-test.coverprofile=/tmp/pkg-rekor-cli." + RandomSuffix(8) + ".cov"
}

func stripCoverageOutput(out string) string {
	return strings.Split(strings.Split(out, "PASS")[0], "FAIL")[0]
}

// RandomSuffix returns a random string of the given length.
func RandomSuffix(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// RandomData returns a random byte slice of the given size.
func RandomData(t *testing.T, n int) []byte {
	t.Helper()
	rand.Seed(time.Now().UnixNano())
	data := make([]byte, n)
	if _, err := rand.Read(data[:]); err != nil {
		t.Fatal(err)
	}
	return data
}

func CreateArtifact(t *testing.T, artifactPath string) string {
	t.Helper()
	// First let's generate some random data so we don't have to worry about dupes.
	data := RandomData(t, 100)

	artifact := base64.StdEncoding.EncodeToString(data[:])
	// Write this to a file
	write(t, artifact, artifactPath)
	return artifact
}

func extractLogEntry(t *testing.T, le models.LogEntry) models.LogEntryAnon {
	t.Helper()

	if len(le) != 1 {
		t.Fatal("expected length to be 1, is actually", len(le))
	}
	for _, v := range le {
		return v
	}
	// this should never happen
	return models.LogEntryAnon{}
}

func write(t *testing.T, data string, path string) {
	t.Helper()
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func GetUUIDFromUploadOutput(t *testing.T, out string) string {
	t.Helper()
	// Output looks like "Artifact timestamped at ...\m Wrote response \n Created entry at index X, available at $URL/UUID", so grab the UUID:
	urlTokens := strings.Split(strings.TrimSpace(out), " ")
	url := urlTokens[len(urlTokens)-1]
	splitUrl := strings.Split(url, "/")
	return splitUrl[len(splitUrl)-1]
}
func SignPGP(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := openpgp.DetachSign(&buf, keys[0], bytes.NewReader(b), nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func Write(t *testing.T, data string, path string) {
	t.Helper()
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

// CreatedPGPSignedArtifact gets the test dir setup correctly with some random artifacts and keys.
func CreatedPGPSignedArtifact(t *testing.T, artifactPath, sigPath string) {
	t.Helper()
	artifact := CreateArtifact(t, artifactPath)

	// Sign it with our key and write that to a file
	signature, err := SignPGP([]byte(artifact))
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(sigPath, signature, 0644); err != nil {
		t.Fatal(err)
	}
}

func GetUUIDFromTimestampOutput(t *testing.T, out string) string {
	t.Helper()
	// Output looks like "Created entry at index X, available at $URL/UUID", so grab the UUID:
	urlTokens := strings.Split(strings.TrimSpace(out), "\n")
	return GetUUIDFromUploadOutput(t, urlTokens[len(urlTokens)-1])
}

// SetupTestData is a helper function to setups the test data
func SetupTestData(t *testing.T) {
	// create a temp directory
	artifactPath := filepath.Join(t.TempDir(), "artifact")
	// create a temp file
	sigPath := filepath.Join(t.TempDir(), "signature.asc")
	CreatedPGPSignedArtifact(t, artifactPath, sigPath)

	// Write the public key to a file
	pubPath := filepath.Join(t.TempDir(), "pubKey.asc")
	if err := ioutil.WriteFile(pubPath, []byte(PubKey), 0644); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	// Now upload to rekor!
	out := RunCli(t, "upload", "--artifact", artifactPath, "--signature", sigPath, "--public-key", pubPath)
	OutputContains(t, out, "Created entry at")
}
