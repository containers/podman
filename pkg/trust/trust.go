package trust

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

// PolicyContent struct for policy.json file
type PolicyContent struct {
	Default    []RepoContent     `json:"default"`
	Transports TransportsContent `json:"transports"`
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

// RegistryConfiguration is one of the files in registriesDirPath configuring lookaside locations, or the result of merging them all.
// NOTE: Keep this in sync with docs/registries.d.md!
type RegistryConfiguration struct {
	DefaultDocker *RegistryNamespace `json:"default-docker"`
	// The key is a namespace, using fully-expanded Docker reference format or parent namespaces (per dockerReference.PolicyConfiguration*),
	Docker map[string]RegistryNamespace `json:"docker"`
}

// RegistryNamespace defines lookaside locations for a single namespace.
type RegistryNamespace struct {
	SigStore        string `json:"sigstore"`         // For reading, and if SigStoreStaging is not present, for writing.
	SigStoreStaging string `json:"sigstore-staging"` // For writing only.
}

// ShowOutput keep the fields for image trust show command
type ShowOutput struct {
	Repo      string
	Trusttype string
	GPGid     string
	Sigstore  string
}

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

// RegistriesDirPath returns a path to registries.d
func RegistriesDirPath(sys *types.SystemContext) string {
	systemRegistriesDirPath := "/etc/containers/registries.d"
	if sys != nil {
		if sys.RegistriesDirPath != "" {
			return sys.RegistriesDirPath
		}
		if sys.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(sys.RootForImplicitAbsolutePaths, systemRegistriesDirPath)
		}
	}
	return systemRegistriesDirPath
}

// LoadAndMergeConfig loads configuration files in dirPath
func LoadAndMergeConfig(dirPath string) (*RegistryConfiguration, error) {
	mergedConfig := RegistryConfiguration{Docker: map[string]RegistryNamespace{}}
	dockerDefaultMergedFrom := ""
	nsMergedFrom := map[string]string{}

	dir, err := os.Open(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &mergedConfig, nil
		}
		return nil, err
	}
	configNames, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for _, configName := range configNames {
		if !strings.HasSuffix(configName, ".yaml") {
			continue
		}
		configPath := filepath.Join(dirPath, configName)
		configBytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		var config RegistryConfiguration
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, errors.Wrapf(err, "Error parsing %s", configPath)
		}
		if config.DefaultDocker != nil {
			if mergedConfig.DefaultDocker != nil {
				return nil, errors.Errorf(`Error parsing signature storage configuration: "default-docker" defined both in "%s" and "%s"`,
					dockerDefaultMergedFrom, configPath)
			}
			mergedConfig.DefaultDocker = config.DefaultDocker
			dockerDefaultMergedFrom = configPath
		}
		for nsName, nsConfig := range config.Docker { // includes config.Docker == nil
			if _, ok := mergedConfig.Docker[nsName]; ok {
				return nil, errors.Errorf(`Error parsing signature storage configuration: "docker" namespace "%s" defined both in "%s" and "%s"`,
					nsName, nsMergedFrom[nsName], configPath)
			}
			mergedConfig.Docker[nsName] = nsConfig
			nsMergedFrom[nsName] = configPath
		}
	}
	return &mergedConfig, nil
}

// HaveMatchRegistry checks if trust settings for the registry have been configed in yaml file
func HaveMatchRegistry(key string, registryConfigs *RegistryConfiguration) *RegistryNamespace {
	searchKey := key
	if !strings.Contains(searchKey, "/") {
		val, exists := registryConfigs.Docker[searchKey]
		if exists {
			return &val
		}
	}
	for range strings.Split(key, "/") {
		val, exists := registryConfigs.Docker[searchKey]
		if exists {
			return &val
		}
		if strings.Contains(searchKey, "/") {
			searchKey = searchKey[:strings.LastIndex(searchKey, "/")]
		}
	}
	return nil
}

// CreateTmpFile creates a temp file under dir and writes the content into it
func CreateTmpFile(dir, pattern string, content []byte) (string, error) {
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

func getGPGIdFromKeyPath(path []string) []string {
	var uids []string
	for _, k := range path {
		cmd := exec.Command("gpg2", "--with-colons", k)
		results, err := cmd.Output()
		if err != nil {
			logrus.Warnf("error get key identity: %s", err)
			continue
		}
		uids = append(uids, parseUids(results)...)
	}
	return uids
}

func getGPGIdFromKeyData(keys []string) []string {
	var uids []string
	for _, k := range keys {
		decodeKey, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			logrus.Warnf("error decoding key data")
			continue
		}
		tmpfileName, err := CreateTmpFile("", "", decodeKey)
		if err != nil {
			logrus.Warnf("error creating key date temp file %s", err)
		}
		defer os.Remove(tmpfileName)
		k = tmpfileName
		cmd := exec.Command("gpg2", "--with-colons", k)
		results, err := cmd.Output()
		if err != nil {
			logrus.Warnf("error get key identity: %s", err)
			continue
		}
		uids = append(uids, parseUids(results)...)
	}
	return uids
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

var typeDescription = map[string]string{"insecureAcceptAnything": "accept", "signedBy": "signed", "reject": "reject"}

func trustTypeDescription(trustType string) string {
	trustDescription, exist := typeDescription[trustType]
	if !exist {
		logrus.Warnf("invalid trust type %s", trustType)
	}
	return trustDescription
}

// GetPolicy return the struct to show policy.json in json format and a map (reponame, ShowOutput) pair for image trust show command
func GetPolicy(policyContentStruct PolicyContent, systemRegistriesDirPath string) (map[string]map[string]interface{}, map[string]ShowOutput, error) {
	registryConfigs, err := LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return nil, nil, err
	}

	trustShowOutputMap := make(map[string]ShowOutput)
	policyJSON := make(map[string]map[string]interface{})
	if len(policyContentStruct.Default) > 0 {
		policyJSON["* (default)"] = make(map[string]interface{})
		policyJSON["* (default)"]["type"] = policyContentStruct.Default[0].Type

		var defaultPolicyStruct ShowOutput
		defaultPolicyStruct.Repo = "default"
		defaultPolicyStruct.Trusttype = trustTypeDescription(policyContentStruct.Default[0].Type)
		trustShowOutputMap["* (default)"] = defaultPolicyStruct
	}
	for transname, transval := range policyContentStruct.Transports {
		for repo, repoval := range transval {
			tempTrustShowOutput := ShowOutput{
				Repo:      repo,
				Trusttype: repoval[0].Type,
			}
			policyJSON[repo] = make(map[string]interface{})
			policyJSON[repo]["type"] = repoval[0].Type
			policyJSON[repo]["transport"] = transname
			keyDataArr := []string{}
			keyPathArr := []string{}
			keyarr := []string{}
			for _, repoele := range repoval {
				if len(repoele.KeyPath) > 0 {
					keyarr = append(keyarr, repoele.KeyPath)
					keyPathArr = append(keyPathArr, repoele.KeyPath)
				}
				if len(repoele.KeyData) > 0 {
					keyarr = append(keyarr, string(repoele.KeyData))
					keyDataArr = append(keyDataArr, string(repoele.KeyData))
				}
			}
			policyJSON[repo]["keys"] = keyarr
			uids := append(getGPGIdFromKeyPath(keyPathArr), getGPGIdFromKeyData(keyDataArr)...)
			tempTrustShowOutput.GPGid = strings.Join(uids, ",")

			policyJSON[repo]["sigstore"] = ""
			registryNamespace := HaveMatchRegistry(repo, registryConfigs)
			if registryNamespace != nil {
				policyJSON[repo]["sigstore"] = registryNamespace.SigStore
				tempTrustShowOutput.Sigstore = registryNamespace.SigStore
			}
			trustShowOutputMap[repo] = tempTrustShowOutput
		}
	}
	return policyJSON, trustShowOutputMap, nil
}
