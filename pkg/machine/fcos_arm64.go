package machine

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	url2 "net/url"

	"github.com/sirupsen/logrus"
)

const aarchBaseURL = "https://fedorapeople.org/groups/fcos-images/builds/latest/aarch64/"

// Total hack until automation is possible.
// We need a proper json file at least to automate
func getFCOSDownload() (*fcosDownloadInfo, error) {
	meta := Build{}
	resp, err := http.Get(aarchBaseURL + "meta.json")
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}
	pathURL, err := url2.Parse(meta.BuildArtifacts.Qemu.Path)
	if err != nil {
		return nil, err
	}

	baseURL, err := url2.Parse(aarchBaseURL)
	if err != nil {
		return nil, err
	}
	pullURL := baseURL.ResolveReference(pathURL)
	return &fcosDownloadInfo{
		Location:  pullURL.String(),
		Release:   "",
		Sha256Sum: meta.BuildArtifacts.Qemu.Sha256,
	}, nil
}

/*
	All of this can be nuked when fcos upstream generates a proper meta data file for aarch.
*/
type AliyunImage struct {
	ImageID string `json:"id"`
	Region  string `json:"name"`
}

type Amis struct {
	Hvm      string `json:"hvm"`
	Region   string `json:"name"`
	Snapshot string `json:"snapshot"`
}

type Artifact struct {
	Path               string  `json:"path"`
	Sha256             string  `json:"sha256"`
	SizeInBytes        float64 `json:"size,omitempty"`
	UncompressedSha256 string  `json:"uncompressed-sha256,omitempty"`
	UncompressedSize   int     `json:"uncompressed-size,omitempty"`
}

type Build struct {
	AlibabaAliyunUploads      []AliyunImage         `json:"aliyun,omitempty"`
	Amis                      []Amis                `json:"amis,omitempty"`
	Architecture              string                `json:"coreos-assembler.basearch,omitempty"`
	Azure                     *Cloudartifact        `json:"azure,omitempty"`
	BuildArtifacts            *BuildArtifacts       `json:"images,omitempty"`
	BuildID                   string                `json:"buildid"`
	BuildRef                  string                `json:"ref,omitempty"`
	BuildSummary              string                `json:"summary"`
	BuildTimeStamp            string                `json:"coreos-assembler.build-timestamp,omitempty"`
	BuildURL                  string                `json:"build-url,omitempty"`
	ConfigGitRev              string                `json:"coreos-assembler.config-gitrev,omitempty"`
	ContainerConfigGit        *Git                  `json:"coreos-assembler.container-config-git,omitempty"`
	CoreOsSource              string                `json:"coreos-assembler.code-source,omitempty"`
	CosaContainerImageGit     *Git                  `json:"coreos-assembler.container-image-git,omitempty"`
	CosaDelayedMetaMerge      bool                  `json:"coreos-assembler.delayed-meta-merge,omitempty"`
	CosaImageChecksum         string                `json:"coreos-assembler.image-config-checksum,omitempty"`
	CosaImageVersion          int                   `json:"coreos-assembler.image-genver,omitempty"`
	Extensions                *Extensions           `json:"extensions,omitempty"`
	FedoraCoreOsParentCommit  string                `json:"fedora-coreos.parent-commit,omitempty"`
	FedoraCoreOsParentVersion string                `json:"fedora-coreos.parent-version,omitempty"`
	Gcp                       *Gcp                  `json:"gcp,omitempty"`
	GitDirty                  string                `json:"coreos-assembler.config-dirty,omitempty"`
	ImageInputChecksum        string                `json:"coreos-assembler.image-input-checksum,omitempty"`
	InputHasOfTheRpmOstree    string                `json:"rpm-ostree-inputhash"`
	MetaStamp                 float64               `json:"coreos-assembler.meta-stamp,omitempty"`
	Name                      string                `json:"name"`
	Oscontainer               *Image                `json:"oscontainer,omitempty"`
	OstreeCommit              string                `json:"ostree-commit"`
	OstreeContentBytesWritten int                   `json:"ostree-content-bytes-written,omitempty"`
	OstreeContentChecksum     string                `json:"ostree-content-checksum"`
	OstreeNCacheHits          int                   `json:"ostree-n-cache-hits,omitempty"`
	OstreeNContentTotal       int                   `json:"ostree-n-content-total,omitempty"`
	OstreeNContentWritten     int                   `json:"ostree-n-content-written,omitempty"`
	OstreeNMetadataTotal      int                   `json:"ostree-n-metadata-total,omitempty"`
	OstreeNMetadataWritten    int                   `json:"ostree-n-metadata-written,omitempty"`
	OstreeTimestamp           string                `json:"ostree-timestamp"`
	OstreeVersion             string                `json:"ostree-version"`
	OverridesActive           bool                  `json:"coreos-assembler.overrides-active,omitempty"`
	PkgdiffAgainstParent      PackageSetDifferences `json:"parent-pkgdiff,omitempty"`
	PkgdiffBetweenBuilds      PackageSetDifferences `json:"pkgdiff,omitempty"`
	ReleasePayload            *Image                `json:"release-payload,omitempty"`
}

type BuildArtifacts struct {
	Aliyun        *Artifact `json:"aliyun,omitempty"`
	Aws           *Artifact `json:"aws,omitempty"`
	Azure         *Artifact `json:"azure,omitempty"`
	AzureStack    *Artifact `json:"azurestack,omitempty"`
	Dasd          *Artifact `json:"dasd,omitempty"`
	DigitalOcean  *Artifact `json:"digitalocean,omitempty"`
	Exoscale      *Artifact `json:"exoscale,omitempty"`
	Gcp           *Artifact `json:"gcp,omitempty"`
	IbmCloud      *Artifact `json:"ibmcloud,omitempty"`
	Initramfs     *Artifact `json:"initramfs,omitempty"`
	Iso           *Artifact `json:"iso,omitempty"`
	Kernel        *Artifact `json:"kernel,omitempty"`
	LiveInitramfs *Artifact `json:"live-initramfs,omitempty"`
	LiveIso       *Artifact `json:"live-iso,omitempty"`
	LiveKernel    *Artifact `json:"live-kernel,omitempty"`
	LiveRootfs    *Artifact `json:"live-rootfs,omitempty"`
	Metal         *Artifact `json:"metal,omitempty"`
	Metal4KNative *Artifact `json:"metal4k,omitempty"`
	OpenStack     *Artifact `json:"openstack,omitempty"`
	Ostree        Artifact  `json:"ostree"`
	Qemu          *Artifact `json:"qemu,omitempty"`
	Vmware        *Artifact `json:"vmware,omitempty"`
	Vultr         *Artifact `json:"vultr,omitempty"`
}

type Cloudartifact struct {
	Image string `json:"image"`
	URL   string `json:"url"`
}

type Extensions struct {
	Manifest       map[string]interface{} `json:"manifest"`
	Path           string                 `json:"path"`
	RpmOstreeState string                 `json:"rpm-ostree-state"`
	Sha256         string                 `json:"sha256"`
}

type Gcp struct {
	ImageFamily  string `json:"family,omitempty"`
	ImageName    string `json:"image"`
	ImageProject string `json:"project,omitempty"`
	URL          string `json:"url"`
}

type Git struct {
	Branch string `json:"branch,omitempty"`
	Commit string `json:"commit"`
	Dirty  string `json:"dirty,omitempty"`
	Origin string `json:"origin"`
}

type Image struct {
	Comment string `json:"comment,omitempty"`
	Digest  string `json:"digest"`
	Image   string `json:"image"`
}

type Items interface{}

type PackageSetDifferences []Items
