package server

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func TestGetInfo(t *testing.T) {
	c := libkpod.DefaultConfig()
	c.RootConfig.Storage = "afoobarstorage"
	c.RootConfig.Root = "afoobarroot"
	c.RuntimeConfig.CgroupManager = "systemd"
	apiConfig := APIConfig{}
	s := &Server{
		config: Config{*c, apiConfig},
	}
	ci := s.getInfo()
	if ci.CgroupDriver != "systemd" {
		t.Fatalf("expected 'systemd', got %q", ci.CgroupDriver)
	}
	if ci.StorageDriver != "afoobarstorage" {
		t.Fatalf("expected 'afoobarstorage', got %q", ci.StorageDriver)
	}
	if ci.StorageRoot != "afoobarroot" {
		t.Fatalf("expected 'afoobarroot', got %q", ci.StorageRoot)
	}
}

type mockNetNS struct {
}

func (ns mockNetNS) Close() error {
	return nil
}
func (ns mockNetNS) Fd() uintptr {
	ptr := new(uintptr)
	return *ptr
}
func (ns mockNetNS) Do(toRun func(ns.NetNS) error) error {
	return nil
}
func (ns mockNetNS) Set() error {
	return nil
}
func (ns mockNetNS) Path() string {
	return ""
}

func TestGetContainerInfo(t *testing.T) {
	s := &Server{}
	created := time.Now()
	labels := map[string]string{
		"io.kubernetes.container.name": "POD",
		"io.kubernetes.test2":          "value2",
		"io.kubernetes.test3":          "value3",
	}
	annotations := map[string]string{
		"io.kubernetes.test":  "value",
		"io.kubernetes.test1": "value1",
	}
	getContainerFunc := func(id string) *oci.Container {
		container, err := oci.NewContainer("testid", "testname", "", "/container/logs", mockNetNS{}, labels, annotations, annotations, "imageName", "imageName", "imageRef", &runtime.ContainerMetadata{}, "testsandboxid", false, false, false, false, false, "/root/for/container", created, "SIGKILL")
		if err != nil {
			t.Fatal(err)
		}
		container.SetMountPoint("/var/foo/container")
		cstate := &oci.ContainerState{}
		cstate.State = specs.State{
			Pid: 42,
		}
		cstate.Created = created
		container.SetState(cstate)
		return container
	}
	getInfraContainerFunc := func(id string) *oci.Container {
		return nil
	}
	getSandboxFunc := func(id string) *sandbox.Sandbox {
		s := &sandbox.Sandbox{}
		s.AddIP("1.1.1.42")
		return s
	}
	ci, err := s.getContainerInfo("", getContainerFunc, getInfraContainerFunc, getSandboxFunc)
	if err != nil {
		t.Fatal(err)
	}
	if ci.CreatedTime != created.UnixNano() {
		t.Fatalf("expected same created time %d, got %d", created.UnixNano(), ci.CreatedTime)
	}
	if ci.Pid != 42 {
		t.Fatalf("expected pid 42, got %v", ci.Pid)
	}
	if ci.Name != "testname" {
		t.Fatalf("expected name testname, got %s", ci.Name)
	}
	if ci.Image != "imageName" {
		t.Fatalf("expected image name imageName, got %s", ci.Image)
	}
	if ci.Root != "/var/foo/container" {
		t.Fatalf("expected root to be /var/foo/container, got %s", ci.Root)
	}
	if ci.LogPath != "/container/logs" {
		t.Fatalf("expected log path to be /containers/logs, got %s", ci.LogPath)
	}
	if ci.Sandbox != "testsandboxid" {
		t.Fatalf("expected sandbox to be testsandboxid, got %s", ci.Sandbox)
	}
	if ci.IP != "1.1.1.42" {
		t.Fatalf("expected ip 1.1.1.42, got %s", ci.IP)
	}
	if len(ci.Annotations) == 0 {
		t.Fatal("annotations are empty")
	}
	if len(ci.Labels) == 0 {
		t.Fatal("labels are empty")
	}
	if len(ci.Annotations) != len(annotations) {
		t.Fatalf("container info annotations len (%d) isn't the same as original annotations len (%d)", len(ci.Annotations), len(annotations))
	}
	if len(ci.Labels) != len(labels) {
		t.Fatalf("container info labels len (%d) isn't the same as original labels len (%d)", len(ci.Labels), len(labels))
	}
	var found bool
	for k, v := range annotations {
		found = false
		for key, value := range ci.Annotations {
			if k == key && v == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("key %s with value %v wasn't in container info annotations", k, v)
		}
	}
	for k, v := range labels {
		found = false
		for key, value := range ci.Labels {
			if k == key && v == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("key %s with value %v wasn't in container info labels", k, v)
		}
	}
}

func TestGetContainerInfoCtrNotFound(t *testing.T) {
	s := &Server{}
	getContainerFunc := func(id string) *oci.Container {
		return nil
	}
	getInfraContainerFunc := func(id string) *oci.Container {
		return nil
	}
	getSandboxFunc := func(id string) *sandbox.Sandbox {
		return nil
	}
	_, err := s.getContainerInfo("", getContainerFunc, getInfraContainerFunc, getSandboxFunc)
	if err == nil {
		t.Fatal("expected an error but got nothing")
	}
	if err != errCtrNotFound {
		t.Fatalf("expected errCtrNotFound error, got %v", err)
	}
}

func TestGetContainerInfoCtrStateNil(t *testing.T) {
	s := &Server{}
	created := time.Now()
	labels := map[string]string{}
	annotations := map[string]string{}
	getContainerFunc := func(id string) *oci.Container {
		container, err := oci.NewContainer("testid", "testname", "", "/container/logs", mockNetNS{}, labels, annotations, annotations, "imageName", "imageName", "imageRef", &runtime.ContainerMetadata{}, "testsandboxid", false, false, false, false, false, "/root/for/container", created, "SIGKILL")
		if err != nil {
			t.Fatal(err)
		}
		container.SetMountPoint("/var/foo/container")
		container.SetState(nil)
		return container
	}
	getInfraContainerFunc := func(id string) *oci.Container {
		return nil
	}
	getSandboxFunc := func(id string) *sandbox.Sandbox {
		s := &sandbox.Sandbox{}
		s.AddIP("1.1.1.42")
		return s
	}
	_, err := s.getContainerInfo("", getContainerFunc, getInfraContainerFunc, getSandboxFunc)
	if err == nil {
		t.Fatal("expected an error but got nothing")
	}
	if err != errCtrStateNil {
		t.Fatalf("expected errCtrStateNil error, got %v", err)
	}
}

func TestGetContainerInfoSandboxNotFound(t *testing.T) {
	s := &Server{}
	created := time.Now()
	labels := map[string]string{}
	annotations := map[string]string{}
	getContainerFunc := func(id string) *oci.Container {
		container, err := oci.NewContainer("testid", "testname", "", "/container/logs", mockNetNS{}, labels, annotations, annotations, "imageName", "imageName", "imageRef", &runtime.ContainerMetadata{}, "testsandboxid", false, false, false, false, false, "/root/for/container", created, "SIGKILL")
		if err != nil {
			t.Fatal(err)
		}
		container.SetMountPoint("/var/foo/container")
		return container
	}
	getInfraContainerFunc := func(id string) *oci.Container {
		return nil
	}
	getSandboxFunc := func(id string) *sandbox.Sandbox {
		return nil
	}
	_, err := s.getContainerInfo("", getContainerFunc, getInfraContainerFunc, getSandboxFunc)
	if err == nil {
		t.Fatal("expected an error but got nothing")
	}
	if err != errSandboxNotFound {
		t.Fatalf("expected errSandboxNotFound error, got %v", err)
	}
}
