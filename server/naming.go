package server

import (
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/stringid"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	kubePrefix    = "k8s"
	infraName     = "POD"
	nameDelimiter = "_"
)

func makeSandboxName(sandboxConfig *pb.PodSandboxConfig) string {
	return strings.Join([]string{
		kubePrefix,
		sandboxConfig.Metadata.Name,
		sandboxConfig.Metadata.Namespace,
		sandboxConfig.Metadata.Uid,
		fmt.Sprintf("%d", sandboxConfig.Metadata.Attempt),
	}, nameDelimiter)
}

func makeSandboxContainerName(sandboxConfig *pb.PodSandboxConfig) string {
	return strings.Join([]string{
		kubePrefix,
		infraName,
		sandboxConfig.Metadata.Name,
		sandboxConfig.Metadata.Namespace,
		sandboxConfig.Metadata.Uid,
		fmt.Sprintf("%d", sandboxConfig.Metadata.Attempt),
	}, nameDelimiter)
}

func makeContainerName(sandboxMetadata *pb.PodSandboxMetadata, containerConfig *pb.ContainerConfig) string {
	return strings.Join([]string{
		kubePrefix,
		containerConfig.Metadata.Name,
		sandboxMetadata.Name,
		sandboxMetadata.Namespace,
		sandboxMetadata.Uid,
		fmt.Sprintf("%d", containerConfig.Metadata.Attempt),
	}, nameDelimiter)
}

func (s *Server) generatePodIDandName(sandboxConfig *pb.PodSandboxConfig) (string, string, error) {
	var (
		err error
		id  = stringid.GenerateNonCryptoID()
	)
	if sandboxConfig.Metadata.Namespace == "" {
		return "", "", fmt.Errorf("cannot generate pod ID without namespace")
	}
	name, err := s.ReservePodName(id, makeSandboxName(sandboxConfig))
	if err != nil {
		return "", "", err
	}
	return id, name, err
}

func (s *Server) generateContainerIDandNameForSandbox(sandboxConfig *pb.PodSandboxConfig) (string, string, error) {
	var (
		err error
		id  = stringid.GenerateNonCryptoID()
	)
	name, err := s.ReserveContainerName(id, makeSandboxContainerName(sandboxConfig))
	if err != nil {
		return "", "", err
	}
	return id, name, err
}

func (s *Server) generateContainerIDandName(sandboxMetadata *pb.PodSandboxMetadata, containerConfig *pb.ContainerConfig) (string, string, error) {
	var (
		err error
		id  = stringid.GenerateNonCryptoID()
	)
	name, err := s.ReserveContainerName(id, makeContainerName(sandboxMetadata, containerConfig))
	if err != nil {
		return "", "", err
	}
	return id, name, err
}
