package defaultnet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// TODO: A smarter implementation would make sure cni-podman0 was unused before
// making the default, and adjust if necessary
const networkTemplate = `{
  "cniVersion": "0.4.0",
  "name": "{{{{.Name}}}}",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "cni-podman0",
      "isGateway": true,
      "ipMasq": true,
      "hairpinMode": true,
      "ipam": {
        "type": "host-local",
        "routes": [{ "dst": "0.0.0.0/0" }],
        "ranges": [
          [
            {
              "subnet": "{{{{.Subnet}}}}",
              "gateway": "{{{{.Gateway}}}}"
            }
          ]
        ]
      }
    },
{{{{- if (eq .Machine true) }}}}
    {
      "type": "podman-machine",
      "capabilities": {
        "portMappings": true
      }
    },
{{{{- end}}}}
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    },
    {
      "type": "firewall"
    },
    {
      "type": "tuning"
    }
  ]
}
`

var (
	// Borrowed from Podman, modified to remove dashes and periods.
	nameRegex = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_]*$")
)

// Used to pass info into the template engine
type networkInfo struct {
	Name    string
	Subnet  string
	Gateway string
	Machine bool
}

// The most trivial definition of a CNI network possible for our use here.
// We need the name, and nothing else.
type network struct {
	Name string `json:"name"`
}

// Create makes the CNI default network, if necessary.
// Accepts the name and subnet of the network to create (a standard template
// will be used, with these values plugged in), the configuration directory
// where CNI configs are stored (to verify if a named configuration already
// exists), an exists directory (where a sentinel file will be stored, to ensure
// the network is only made once), and an isMachine bool (to determine whether
// the machine block will be added to the config).
// Create first checks if a default network has already been created via the
// presence of a sentinel file. If it does exist, it returns immediately without
// error.
// It next checks if a CNI network with the given name already exists. In that
// case, it creates the sentinel file and returns without error.
// If neither of these are true, the default network is created.
func Create(name, subnet, configDir, existsDir string, isMachine bool) error {
	// TODO: Should probably regex name to make sure it's valid.
	if name == "" || subnet == "" || configDir == "" || existsDir == "" {
		return errors.Errorf("must provide values for all arguments to MakeDefaultNetwork")
	}
	if !nameRegex.MatchString(name) {
		return errors.Errorf("invalid default network name %s - letters, numbers, and underscores only", name)
	}

	sentinelFile := filepath.Join(existsDir, "defaultCNINetExists")

	// Check if sentinel file exists, return immediately if it does.
	if _, err := os.Stat(sentinelFile); err == nil {
		return nil
	}

	// Create the sentinel file if it doesn't exist, so subsequent checks
	// don't need to go further.
	file, err := os.Create(sentinelFile)
	if err != nil {
		return err
	}
	file.Close()

	// We may need to make the config dir.
	if err := os.MkdirAll(configDir, 0755); err != nil && !os.IsExist(err) {
		return errors.Wrapf(err, "error creating CNI configuration directory")
	}

	// Check all networks in the CNI conflist.
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		return errors.Wrapf(err, "error reading CNI configuration directory")
	}
	if len(files) > 0 {
		configPaths := make([]string, 0, len(files))
		for _, path := range files {
			if !path.IsDir() && filepath.Ext(path.Name()) == ".conflist" {
				configPaths = append(configPaths, filepath.Join(configDir, path.Name()))
			}
		}
		for _, config := range configPaths {
			configName, err := getConfigName(config)
			if err != nil {
				logrus.Errorf("Error reading CNI configuration file: %v", err)
				continue
			}
			if configName == name {
				return nil
			}
		}
	}

	// We need to make the config.
	// Get subnet and gateway.
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return errors.Wrapf(err, "default network subnet %s is invalid", subnet)
	}

	ones, bits := ipNet.Mask.Size()
	if ones == bits {
		return errors.Wrapf(err, "default network subnet %s is to small", subnet)
	}
	gateway := make(net.IP, len(ipNet.IP))
	// copy the subnet ip to the gateway so we can modify it
	copy(gateway, ipNet.IP)
	// the default gateway should be the first ip in the subnet
	gateway[len(gateway)-1]++

	netInfo := new(networkInfo)
	netInfo.Name = name
	netInfo.Gateway = gateway.String()
	netInfo.Subnet = ipNet.String()
	netInfo.Machine = isMachine

	templ, err := template.New("network_template").Delims("{{{{", "}}}}").Parse(networkTemplate)
	if err != nil {
		return errors.Wrapf(err, "error compiling template for default network")
	}
	var output bytes.Buffer
	if err := templ.Execute(&output, netInfo); err != nil {
		return errors.Wrapf(err, "error executing template for default network")
	}

	// Next, we need to place the config on disk.
	// Loop through possible indexes, with a limit of 100 attempts.
	created := false
	for i := 87; i < 187; i++ {
		configFile, err := os.OpenFile(filepath.Join(configDir, fmt.Sprintf("%d-%s.conflist", i, name)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			logrus.Infof("Attempt to create default CNI network config file failed: %v", err)
			continue
		}
		defer configFile.Close()

		created = true

		// Success - file is open. Write our buffer to it.
		if _, err := configFile.Write(output.Bytes()); err != nil {
			return errors.Wrapf(err, "error writing default CNI config to file")
		}
		break
	}
	if !created {
		return errors.Errorf("no available default network configuration file was found")
	}

	return nil
}

// Get the name of the configuration contained in a given conflist file. Accepts
// the full path of a .conflist CNI configuration.
func getConfigName(file string) (string, error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	config := new(network)
	if err := json.Unmarshal(contents, config); err != nil {
		return "", errors.Wrapf(err, "error decoding CNI configuration %s", filepath.Base(file))
	}
	return config.Name, nil
}
