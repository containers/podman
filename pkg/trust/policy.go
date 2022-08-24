package trust

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
)

// PolicyContent struct for policy.json file
type PolicyContent struct {
	Default    []RepoContent     `json:"default"`
	Transports TransportsContent `json:"transports,omitempty"`
}

// RepoContent struct used under each repo
type RepoContent struct {
	Type           string          `json:"type"`
	KeyType        string          `json:"keyType,omitempty"`
	KeyPath        string          `json:"keyPath,omitempty"`
	KeyData        string          `json:"keyData,omitempty"`
	SignedIdentity json.RawMessage `json:"signedIdentity,omitempty"`
}

// RepoMap map repo name to policycontent for each repo
type RepoMap map[string][]RepoContent

// TransportsContent struct for content under "transports"
type TransportsContent map[string]RepoMap

// DefaultPolicyPath returns a path to the default policy of the system.
func DefaultPolicyPath(sys *types.SystemContext) string {
	systemDefaultPolicyPath := "/etc/containers/policy.json"
	if sys != nil {
		if sys.SignaturePolicyPath != "" {
			return sys.SignaturePolicyPath
		}
		if sys.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(sys.RootForImplicitAbsolutePaths, systemDefaultPolicyPath)
		}
	}
	return systemDefaultPolicyPath
}

// createTmpFile creates a temp file under dir and writes the content into it
func createTmpFile(dir, pattern string, content []byte) (string, error) {
	tmpfile, err := ioutil.TempFile(dir, pattern)
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	if _, err := tmpfile.Write(content); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

// GetGPGIdFromKeyPath return user keyring from key path
func GetGPGIdFromKeyPath(path string) []string {
	cmd := exec.Command("gpg2", "--with-colons", path)
	results, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Getting key identity: %s", err)
		return nil
	}
	return parseUids(results)
}

// GetGPGIdFromKeyData return user keyring from keydata
func GetGPGIdFromKeyData(key string) []string {
	decodeKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		logrus.Errorf("%s, error decoding key data", err)
		return nil
	}
	tmpfileName, err := createTmpFile("", "", decodeKey)
	if err != nil {
		logrus.Errorf("Creating key date temp file %s", err)
	}
	defer os.Remove(tmpfileName)
	return GetGPGIdFromKeyPath(tmpfileName)
}

func parseUids(colonDelimitKeys []byte) []string {
	var parseduids []string
	scanner := bufio.NewScanner(bytes.NewReader(colonDelimitKeys))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "uid:") || strings.HasPrefix(line, "pub:") {
			uid := strings.Split(line, ":")[9]
			if uid == "" {
				continue
			}
			parseduid := uid
			if strings.Contains(uid, "<") && strings.Contains(uid, ">") {
				parseduid = strings.SplitN(strings.SplitAfterN(uid, "<", 2)[1], ">", 2)[0]
			}
			parseduids = append(parseduids, parseduid)
		}
	}
	return parseduids
}

// GetPolicy parse policy.json into PolicyContent struct
func GetPolicy(policyPath string) (PolicyContent, error) {
	var policyContentStruct PolicyContent
	policyContent, err := ioutil.ReadFile(policyPath)
	if err != nil {
		return policyContentStruct, fmt.Errorf("unable to read policy file: %w", err)
	}
	if err := json.Unmarshal(policyContent, &policyContentStruct); err != nil {
		return policyContentStruct, fmt.Errorf("could not parse trust policies from %s: %w", policyPath, err)
	}
	return policyContentStruct, nil
}
