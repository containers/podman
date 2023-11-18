package config

import (
	"github.com/containers/image/v5/manifest"
	dockerclient "github.com/fsouza/go-dockerclient"
)

// Schema2ConfigFromGoDockerclientConfig converts a go-dockerclient Config
// structure to a manifest Schema2Config.
func Schema2ConfigFromGoDockerclientConfig(config *dockerclient.Config) *manifest.Schema2Config {
	overrideExposedPorts := make(map[manifest.Schema2Port]struct{})
	for port := range config.ExposedPorts {
		overrideExposedPorts[manifest.Schema2Port(port)] = struct{}{}
	}
	var overrideHealthCheck *manifest.Schema2HealthConfig
	if config.Healthcheck != nil {
		overrideHealthCheck = &manifest.Schema2HealthConfig{
			Test:        config.Healthcheck.Test,
			StartPeriod: config.Healthcheck.StartPeriod,
			Interval:    config.Healthcheck.Interval,
			Timeout:     config.Healthcheck.Timeout,
			Retries:     config.Healthcheck.Retries,
		}
	}
	labels := make(map[string]string)
	for k, v := range config.Labels {
		labels[k] = v
	}
	volumes := make(map[string]struct{})
	for v := range config.Volumes {
		volumes[v] = struct{}{}
	}
	s2config := &manifest.Schema2Config{
		Hostname:        config.Hostname,
		Domainname:      config.Domainname,
		User:            config.User,
		AttachStdin:     config.AttachStdin,
		AttachStdout:    config.AttachStdout,
		AttachStderr:    config.AttachStderr,
		ExposedPorts:    overrideExposedPorts,
		Tty:             config.Tty,
		OpenStdin:       config.OpenStdin,
		StdinOnce:       config.StdinOnce,
		Env:             append([]string{}, config.Env...),
		Cmd:             append([]string{}, config.Cmd...),
		Healthcheck:     overrideHealthCheck,
		ArgsEscaped:     config.ArgsEscaped,
		Image:           config.Image,
		Volumes:         volumes,
		WorkingDir:      config.WorkingDir,
		Entrypoint:      append([]string{}, config.Entrypoint...),
		NetworkDisabled: config.NetworkDisabled,
		MacAddress:      config.MacAddress,
		OnBuild:         append([]string{}, config.OnBuild...),
		Labels:          labels,
		StopSignal:      config.StopSignal,
		Shell:           config.Shell,
	}
	if config.StopTimeout != 0 {
		s2config.StopTimeout = &config.StopTimeout
	}
	return s2config
}

// GoDockerclientConfigFromSchema2Config converts a manifest Schema2Config
// to a go-dockerclient config structure.
func GoDockerclientConfigFromSchema2Config(s2config *manifest.Schema2Config) *dockerclient.Config {
	overrideExposedPorts := make(map[dockerclient.Port]struct{})
	for port := range s2config.ExposedPorts {
		overrideExposedPorts[dockerclient.Port(port)] = struct{}{}
	}
	var healthCheck *dockerclient.HealthConfig
	if s2config.Healthcheck != nil {
		healthCheck = &dockerclient.HealthConfig{
			Test:        s2config.Healthcheck.Test,
			StartPeriod: s2config.Healthcheck.StartPeriod,
			Interval:    s2config.Healthcheck.Interval,
			Timeout:     s2config.Healthcheck.Timeout,
			Retries:     s2config.Healthcheck.Retries,
		}
	}
	labels := make(map[string]string)
	for k, v := range s2config.Labels {
		labels[k] = v
	}
	volumes := make(map[string]struct{})
	for v := range s2config.Volumes {
		volumes[v] = struct{}{}
	}
	config := &dockerclient.Config{
		Hostname:        s2config.Hostname,
		Domainname:      s2config.Domainname,
		User:            s2config.User,
		AttachStdin:     s2config.AttachStdin,
		AttachStdout:    s2config.AttachStdout,
		AttachStderr:    s2config.AttachStderr,
		PortSpecs:       nil,
		ExposedPorts:    overrideExposedPorts,
		Tty:             s2config.Tty,
		OpenStdin:       s2config.OpenStdin,
		StdinOnce:       s2config.StdinOnce,
		Env:             append([]string{}, s2config.Env...),
		Cmd:             append([]string{}, s2config.Cmd...),
		Healthcheck:     healthCheck,
		ArgsEscaped:     s2config.ArgsEscaped,
		Image:           s2config.Image,
		Volumes:         volumes,
		WorkingDir:      s2config.WorkingDir,
		Entrypoint:      append([]string{}, s2config.Entrypoint...),
		NetworkDisabled: s2config.NetworkDisabled,
		MacAddress:      s2config.MacAddress,
		OnBuild:         append([]string{}, s2config.OnBuild...),
		Labels:          labels,
		StopSignal:      s2config.StopSignal,
		Shell:           s2config.Shell,
	}
	if s2config.StopTimeout != nil {
		config.StopTimeout = *s2config.StopTimeout
	}
	return config
}
