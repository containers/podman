// +build !remoteclient

package adapter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/network"
	"github.com/pkg/errors"
)

func getCNIConfDir(r *LocalRuntime) (string, error) {
	config, err := r.GetConfig()
	if err != nil {
		return "", err
	}
	configPath := config.CNIConfigDir

	if len(config.CNIConfigDir) < 1 {
		configPath = network.CNIConfigDir
	}
	return configPath, nil
}

// NetworkList displays summary information about CNI networks
func (r *LocalRuntime) NetworkList(cli *cliconfig.NetworkListValues) error {
	cniConfigPath, err := getCNIConfDir(r)
	if err != nil {
		return err
	}
	networks, err := network.LoadCNIConfsFromDir(cniConfigPath)
	if err != nil {
		return err
	}
	// quiet means we only print the network names
	if cli.Quiet {
		for _, cniNetwork := range networks {
			fmt.Println(cniNetwork.Name)
		}
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tVERSION\tPLUGINS"); err != nil {
		return err
	}
	for _, cniNetwork := range networks {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", cniNetwork.Name, cniNetwork.CNIVersion, getCNIPlugins(cniNetwork)); err != nil {
			return err
		}
	}
	return w.Flush()
}

// NetworkInspect displays the raw CNI configuration for one
// or more CNI networks
func (r *LocalRuntime) NetworkInspect(cli *cliconfig.NetworkInspectValues) error {
	var (
		rawCNINetworks []map[string]interface{}
	)
	cniConfigPath, err := getCNIConfDir(r)
	if err != nil {
		return err
	}
	for _, name := range cli.InputArgs {
		b, err := readRawCNIConfByName(name, cniConfigPath)
		if err != nil {
			return err
		}
		rawList := make(map[string]interface{})
		if err := json.Unmarshal(b, &rawList); err != nil {
			return fmt.Errorf("error parsing configuration list: %s", err)
		}
		rawCNINetworks = append(rawCNINetworks, rawList)
	}
	out, err := json.MarshalIndent(rawCNINetworks, "", "\t")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

// NetworkRemove deletes one or more CNI networks
func (r *LocalRuntime) NetworkRemove(cli *cliconfig.NetworkRmValues) error {
	cniConfigPath, err := getCNIConfDir(r)
	if err != nil {
		return err
	}
	for _, name := range cli.InputArgs {
		cniPath, err := getCNIConfigPathByName(name, cniConfigPath)
		if err != nil {
			return err
		}
		if err := os.Remove(cniPath); err != nil {
			return err
		}
		fmt.Printf("Deleted: %s\n", name)
	}
	return nil
}

// getCNIConfigPathByName finds a CNI network by name and
// returns its configuration file path
func getCNIConfigPathByName(name, cniConfigPath string) (string, error) {
	files, err := libcni.ConfFiles(cniConfigPath, []string{".conflist"})
	if err != nil {
		return "", err
	}
	for _, confFile := range files {
		conf, err := libcni.ConfListFromFile(confFile)
		if err != nil {
			return "", err
		}
		if conf.Name == name {
			return confFile, nil
		}
	}
	return "", errors.Errorf("unable to find network configuration for %s", name)
}

// readRawCNIConfByName reads the raw CNI configuration for a CNI
// network by name
func readRawCNIConfByName(name, cniConfigPath string) ([]byte, error) {
	confFile, err := getCNIConfigPathByName(name, cniConfigPath)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(confFile)
	return b, err
}

// getCNIPlugins returns a list of plugins that a given network
// has in the form of a string
func getCNIPlugins(list *libcni.NetworkConfigList) string {
	var plugins []string
	for _, plug := range list.Plugins {
		plugins = append(plugins, plug.Network.Type)
	}
	return strings.Join(plugins, ",")
}
