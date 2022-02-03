module github.com/openshift/imagebuilder

go 1.16

require (
	github.com/containerd/containerd v1.5.9
	github.com/containers/storage v1.37.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11 // indirect
	github.com/fsouza/go-dockerclient v1.7.7
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20220114011407-0dd24b26b47d // indirect
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9 // indirect
	k8s.io/klog v1.0.0
)
