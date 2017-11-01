package server

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/syndtr/gocapability/capability"
)

const (
	// According to http://man7.org/linux/man-pages/man5/resolv.conf.5.html:
	// "The search list is currently limited to six domains with a total of 256 characters."
	maxDNSSearches = 6
)

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func removeFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func parseDNSOptions(servers, searches, options []string, path string) error {
	nServers := len(servers)
	nSearches := len(searches)
	nOptions := len(options)
	if nServers == 0 && nSearches == 0 && nOptions == 0 {
		return copyFile("/etc/resolv.conf", path)
	}

	if nSearches > maxDNSSearches {
		return fmt.Errorf("DNSOption.Searches has more than 6 domains")
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if nSearches > 0 {
		data := fmt.Sprintf("search %s\n", strings.Join(searches, " "))
		_, err = f.Write([]byte(data))
		if err != nil {
			return err
		}
	}

	if nServers > 0 {
		data := fmt.Sprintf("nameserver %s\n", strings.Join(servers, "\nnameserver "))
		_, err = f.Write([]byte(data))
		if err != nil {
			return err
		}
	}

	if nOptions > 0 {
		data := fmt.Sprintf("options %s\n", strings.Join(options, " "))
		_, err = f.Write([]byte(data))
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: remove sysctl extraction related code here, instead we import from k8s directly.

const (
	// SysctlsPodAnnotationKey represents the key of sysctls which are set for the infrastructure
	// container of a pod. The annotation value is a comma separated list of sysctl_name=value
	// key-value pairs. Only a limited set of whitelisted and isolated sysctls is supported by
	// the kubelet. Pods with other sysctls will fail to launch.
	SysctlsPodAnnotationKey string = "security.alpha.kubernetes.io/sysctls"

	// UnsafeSysctlsPodAnnotationKey represents the key of sysctls which are set for the infrastructure
	// container of a pod. The annotation value is a comma separated list of sysctl_name=value
	// key-value pairs. Unsafe sysctls must be explicitly enabled for a kubelet. They are properly
	// namespaced to a pod or a container, but their isolation is usually unclear or weak. Their use
	// is at-your-own-risk. Pods that attempt to set an unsafe sysctl that is not enabled for a kubelet
	// will fail to launch.
	UnsafeSysctlsPodAnnotationKey string = "security.alpha.kubernetes.io/unsafe-sysctls"
)

// Sysctl defines a kernel parameter to be set
type Sysctl struct {
	// Name of a property to set
	Name string `json:"name"`
	// Value of a property to set
	Value string `json:"value"`
}

// SysctlsFromPodAnnotations parses the sysctl annotations into a slice of safe Sysctls
// and a slice of unsafe Sysctls. This is only a convenience wrapper around
// SysctlsFromPodAnnotation.
func SysctlsFromPodAnnotations(a map[string]string) ([]Sysctl, []Sysctl, error) {
	safe, err := SysctlsFromPodAnnotation(a[SysctlsPodAnnotationKey])
	if err != nil {
		return nil, nil, err
	}
	unsafe, err := SysctlsFromPodAnnotation(a[UnsafeSysctlsPodAnnotationKey])
	if err != nil {
		return nil, nil, err
	}

	return safe, unsafe, nil
}

// SysctlsFromPodAnnotation parses an annotation value into a slice of Sysctls.
func SysctlsFromPodAnnotation(annotation string) ([]Sysctl, error) {
	if len(annotation) == 0 {
		return nil, nil
	}

	kvs := strings.Split(annotation, ",")
	sysctls := make([]Sysctl, len(kvs))
	for i, kv := range kvs {
		cs := strings.Split(kv, "=")
		if len(cs) != 2 || len(cs[0]) == 0 {
			return nil, fmt.Errorf("sysctl %q not of the format sysctl_name=value", kv)
		}
		sysctls[i].Name = cs[0]
		sysctls[i].Value = cs[1]
	}
	return sysctls, nil
}

func newPodNetwork(sb *sandbox.Sandbox) ocicni.PodNetwork {
	return ocicni.PodNetwork{
		Name:      sb.KubeName(),
		Namespace: sb.Namespace(),
		ID:        sb.ID(),
		NetNS:     sb.NetNsPath(),
	}
}

// inStringSlice checks whether a string is inside a string slice.
// Comparison is case insensitive.
func inStringSlice(ss []string, str string) bool {
	for _, s := range ss {
		if strings.ToLower(s) == strings.ToLower(str) {
			return true
		}
	}
	return false
}

// getOCICapabilitiesList returns a list of all available capabilities.
func getOCICapabilitiesList() []string {
	var caps []string
	for _, cap := range capability.List() {
		if cap > validate.LastCap() {
			continue
		}
		caps = append(caps, "CAP_"+strings.ToUpper(cap.String()))
	}
	return caps
}
