package server

import (
	"bytes"
	"io/ioutil"

	"github.com/BurntSushi/toml"
	"github.com/kubernetes-incubator/cri-o/libkpod"
)

//CrioConfigPath is the default location for the conf file
const CrioConfigPath = "/etc/crio/crio.conf"

// Config represents the entire set of configuration values that can be set for
// the server. This is intended to be loaded from a toml-encoded config file.
type Config struct {
	libkpod.Config
	APIConfig
}

// APIConfig represents the "crio.api" TOML config table.
type APIConfig struct {
	// Listen is the path to the AF_LOCAL socket on which cri-o will listen.
	// This may support proto://addr formats later, but currently this is just
	// a path.
	Listen string `toml:"listen"`

	// StreamAddress is the IP address on which the stream server will listen.
	StreamAddress string `toml:"stream_address"`

	// StreamPort is the port on which the stream server will listen.
	StreamPort string `toml:"stream_port"`
}

// tomlConfig is another way of looking at a Config, which is
// TOML-friendly (it has all of the explicit tables). It's just used for
// conversions.
type tomlConfig struct {
	Crio struct {
		libkpod.RootConfig
		API     struct{ APIConfig }             `toml:"api"`
		Runtime struct{ libkpod.RuntimeConfig } `toml:"runtime"`
		Image   struct{ libkpod.ImageConfig }   `toml:"image"`
		Network struct{ libkpod.NetworkConfig } `toml:"network"`
	} `toml:"crio"`
}

func (t *tomlConfig) toConfig(c *Config) {
	c.RootConfig = t.Crio.RootConfig
	c.APIConfig = t.Crio.API.APIConfig
	c.RuntimeConfig = t.Crio.Runtime.RuntimeConfig
	c.ImageConfig = t.Crio.Image.ImageConfig
	c.NetworkConfig = t.Crio.Network.NetworkConfig
}

func (t *tomlConfig) fromConfig(c *Config) {
	t.Crio.RootConfig = c.RootConfig
	t.Crio.API.APIConfig = c.APIConfig
	t.Crio.Runtime.RuntimeConfig = c.RuntimeConfig
	t.Crio.Image.ImageConfig = c.ImageConfig
	t.Crio.Network.NetworkConfig = c.NetworkConfig
}

// UpdateFromFile populates the Config from the TOML-encoded file at the given path.
// Returns errors encountered when reading or parsing the files, or nil
// otherwise.
func (c *Config) UpdateFromFile(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	t := new(tomlConfig)
	t.fromConfig(c)

	_, err = toml.Decode(string(data), t)
	if err != nil {
		return err
	}

	t.toConfig(c)
	return nil
}

// ToFile outputs the given Config as a TOML-encoded file at the given path.
// Returns errors encountered when generating or writing the file, or nil
// otherwise.
func (c *Config) ToFile(path string) error {
	var w bytes.Buffer
	e := toml.NewEncoder(&w)

	t := new(tomlConfig)
	t.fromConfig(c)

	if err := e.Encode(*t); err != nil {
		return err
	}

	return ioutil.WriteFile(path, w.Bytes(), 0644)
}

// DefaultConfig returns the default configuration for crio.
func DefaultConfig() *Config {
	return &Config{
		Config: *libkpod.DefaultConfig(),
		APIConfig: APIConfig{
			Listen:        "/var/run/crio.sock",
			StreamAddress: "",
			StreamPort:    "10010",
		},
	}
}
