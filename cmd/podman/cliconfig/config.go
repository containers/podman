package cliconfig

import (
	"net"

	"github.com/spf13/cobra"
)

type PodmanCommand struct {
	*cobra.Command
	InputArgs   []string
	GlobalFlags MainFlags
	Remote      bool
}

type MainFlags struct {
	CGroupManager     string
	CniConfigDir      string
	ConmonPath        string
	DefaultMountsFile string
	EventsBackend     string
	HooksDir          []string
	MaxWorks          int
	Namespace         string
	Root              string
	Runroot           string
	Runtime           string
	StorageDriver     string
	StorageOpts       []string
	Syslog            bool
	Trace             bool
	NetworkCmdPath    string

	Config     string
	CpuProfile string
	LogLevel   string
	TmpDir     string

	RemoteUserName       string
	RemoteHost           string
	VarlinkAddress       string
	ConnectionName       string
	RemoteConfigFilePath string
	Port                 int
	IdentityFile         string
	IgnoreHosts          bool
}

type AttachValues struct {
	PodmanCommand
	DetachKeys string
	Latest     bool
	NoStdin    bool
	SigProxy   bool
}

type ImagesValues struct {
	PodmanCommand
	All       bool
	Digests   bool
	Filter    []string
	Format    string
	Noheading bool
	NoTrunc   bool
	Quiet     bool
	Sort      string
}

type EventValues struct {
	PodmanCommand
	Filter []string
	Format string
	Since  string
	Stream bool
	Until  string
}

type TagValues struct {
	PodmanCommand
}

type TreeValues struct {
	PodmanCommand
	WhatRequires bool
}

type WaitValues struct {
	PodmanCommand
	Interval uint
	Latest   bool
}

type CheckpointValues struct {
	PodmanCommand
	Keep           bool
	LeaveRunning   bool
	TcpEstablished bool
	All            bool
	Latest         bool
	Export         string
	IgnoreRootfs   bool
}

type CommitValues struct {
	PodmanCommand
	Change         []string
	Format         string
	Message        string
	Author         string
	Pause          bool
	Quiet          bool
	IncludeVolumes bool
}

type ContainersPrune struct {
	PodmanCommand
}

type DiffValues struct {
	PodmanCommand
	Archive bool
	Format  string
	Latest  bool
}

type ExecValues struct {
	PodmanCommand
	DetachKeys  string
	Env         []string
	Privileged  bool
	Interactive bool
	Tty         bool
	User        string
	Latest      bool
	Workdir     string
	PreserveFDs int
}

type ImageExistsValues struct {
	PodmanCommand
}

type ContainerExistsValues struct {
	PodmanCommand
}

type PodExistsValues struct {
	PodmanCommand
}

type ExportValues struct {
	PodmanCommand
	Output string
}
type GenerateKubeValues struct {
	PodmanCommand
	Service  bool
	Filename string
}

type GenerateSystemdValues struct {
	PodmanCommand
	Name          bool
	Files         bool
	RestartPolicy string
	StopTimeout   int
}

type HistoryValues struct {
	PodmanCommand
	Human   bool
	NoTrunc bool
	Quiet   bool
	Format  string
}
type PruneImagesValues struct {
	PodmanCommand
	All bool
}

type PruneContainersValues struct {
	PodmanCommand
	Force bool
}

type PodPruneValues struct {
	PodmanCommand
	Force bool
}

type ImportValues struct {
	PodmanCommand
	Change  []string
	Message string
	Quiet   bool
}

type InfoValues struct {
	PodmanCommand
	Debug  bool
	Format string
}

type InitValues struct {
	PodmanCommand
	All    bool
	Latest bool
}

type InspectValues struct {
	PodmanCommand
	TypeObject string
	Format     string
	Size       bool
	Latest     bool
}

type KillValues struct {
	PodmanCommand
	All    bool
	Signal string
	Latest bool
}

type LoadValues struct {
	PodmanCommand
	Input           string
	Quiet           bool
	SignaturePolicy string
}

type LoginValues struct {
	PodmanCommand
	Password      string
	StdinPassword bool
	Username      string
	Authfile      string
	CertDir       string
	GetLogin      bool
	TlsVerify     bool
}

type LogoutValues struct {
	PodmanCommand
	Authfile string
	All      bool
}

type LogsValues struct {
	PodmanCommand
	Details    bool
	Follow     bool
	Since      string
	Tail       uint64
	Timestamps bool
	Latest     bool
}

type MountValues struct {
	PodmanCommand
	All     bool
	Format  string
	NoTrunc bool
	Latest  bool
}

type NetworkCreateValues struct {
	PodmanCommand
	Driver     string
	DisableDNS bool
	Gateway    net.IP
	Internal   bool
	IPamDriver string
	IPRange    net.IPNet
	IPV6       bool
	Network    net.IPNet
}

type NetworkListValues struct {
	PodmanCommand
	Filter []string
	Quiet  bool
}

type NetworkRmValues struct {
	PodmanCommand
	Force bool
}

type NetworkInspectValues struct {
	PodmanCommand
}

type PauseValues struct {
	PodmanCommand
	All bool
}

type HealthCheckValues struct {
	PodmanCommand
}

type KubePlayValues struct {
	PodmanCommand
	Authfile        string
	CertDir         string
	Creds           string
	Quiet           bool
	SignaturePolicy string
	TlsVerify       bool
}

type PodCreateValues struct {
	PodmanCommand
	CgroupParent string
	Infra        bool
	InfraImage   string
	InfraCommand string
	LabelFile    []string
	Labels       []string
	Name         string
	Hostname     string
	PodIDFile    string
	Publish      []string
	Share        string
}

type PodInspectValues struct {
	PodmanCommand
	Latest bool
}

type PodKillValues struct {
	PodmanCommand
	All    bool
	Signal string
	Latest bool
}

type PodPauseValues struct {
	PodmanCommand
	All    bool
	Latest bool
}

type PodPsValues struct {
	PodmanCommand
	CtrNames  bool
	CtrIDs    bool
	CtrStatus bool
	Filter    string
	Format    string
	Latest    bool
	Namespace bool
	NoTrunc   bool
	Quiet     bool
	Sort      string
}

type PodRestartValues struct {
	PodmanCommand
	All    bool
	Latest bool
}

type PodRmValues struct {
	PodmanCommand
	All    bool
	Force  bool
	Latest bool
}

type PodStartValues struct {
	PodmanCommand
	All    bool
	Latest bool
}
type PodStatsValues struct {
	PodmanCommand
	All      bool
	NoStream bool
	NoReset  bool
	Format   string
	Latest   bool
}

type PodStopValues struct {
	PodmanCommand
	All     bool
	Latest  bool
	Timeout uint
}

type PodTopValues struct {
	PodmanCommand
	Latest          bool
	ListDescriptors bool
}
type PodUnpauseValues struct {
	PodmanCommand
	All    bool
	Latest bool
}

type PortValues struct {
	PodmanCommand
	All    bool
	Latest bool
}

type PsValues struct {
	PodmanCommand
	All       bool
	Filter    []string
	Format    string
	Last      int
	Latest    bool
	Namespace bool
	NoTrunct  bool
	Pod       bool
	Quiet     bool
	Size      bool
	Sort      string
	Sync      bool
	Watch     uint
}

type PullValues struct {
	PodmanCommand
	AllTags         bool
	Authfile        string
	CertDir         string
	Creds           string
	OverrideArch    string
	OverrideOS      string
	Quiet           bool
	SignaturePolicy string
	TlsVerify       bool
}

type PushValues struct {
	PodmanCommand
	Authfile         string
	CertDir          string
	Compress         bool
	Creds            string
	Digestfile       string
	Format           string
	Quiet            bool
	RemoveSignatures bool
	SignBy           string
	SignaturePolicy  string
	TlsVerify        bool
}

type RefreshValues struct {
	PodmanCommand
}

type RestartValues struct {
	PodmanCommand
	All     bool
	Latest  bool
	Running bool
	Timeout uint
}

type RestoreValues struct {
	PodmanCommand
	All            bool
	Keep           bool
	Latest         bool
	TcpEstablished bool
	Import         string
	Name           string
	IgnoreRootfs   bool
	IgnoreStaticIP bool
}

type RmValues struct {
	PodmanCommand
	All     bool
	Force   bool
	Latest  bool
	Storage bool
	Volumes bool
}

type RmiValues struct {
	PodmanCommand
	All   bool
	Force bool
}

type RunlabelValues struct {
	PodmanCommand
	Authfile        string
	CertDir         string
	Creds           string
	Display         bool
	Name            string
	Opt1            string
	Opt2            string
	Opt3            string
	Quiet           bool
	Replace         bool
	SignaturePolicy string
	TlsVerify       bool
}
type SaveValues struct {
	PodmanCommand
	Compress bool
	Format   string
	Output   string
	Quiet    bool
}

type SearchValues struct {
	PodmanCommand
	Authfile  string
	Filter    []string
	Format    string
	Limit     int
	NoTrunc   bool
	TlsVerify bool
}

type TrustValues struct {
	PodmanCommand
}

type SignValues struct {
	PodmanCommand
	Directory string
	SignBy    string
	CertDir   string
}

type StartValues struct {
	PodmanCommand
	Attach      bool
	DetachKeys  string
	Interactive bool
	Latest      bool
	SigProxy    bool
}

type StatsValues struct {
	PodmanCommand
	All      bool
	Format   string
	Latest   bool
	NoReset  bool
	NoStream bool
}

type StopValues struct {
	PodmanCommand
	All     bool
	Latest  bool
	Timeout uint
}

type TopValues struct {
	PodmanCommand
	Latest          bool
	ListDescriptors bool
}

type UmountValues struct {
	PodmanCommand
	All    bool
	Force  bool
	Latest bool
}

type UnpauseValues struct {
	PodmanCommand
	All bool
}

type VarlinkValues struct {
	PodmanCommand
	Timeout int64
}

type SetTrustValues struct {
	PodmanCommand
	PolicyPath  string
	PubKeysFile []string
	TrustType   string
}

type ShowTrustValues struct {
	PodmanCommand
	Json         bool
	PolicyPath   string
	Raw          bool
	RegistryPath string
}

type VersionValues struct {
	PodmanCommand
	Format string
}

type VolumeCreateValues struct {
	PodmanCommand
	Driver string
	Label  []string
	Opt    []string
}
type VolumeInspectValues struct {
	PodmanCommand
	All    bool
	Format string
}

type VolumeLsValues struct {
	PodmanCommand
	Filter string
	Format string
	Quiet  bool
}

type VolumePruneValues struct {
	PodmanCommand
	Force bool
}

type VolumeRmValues struct {
	PodmanCommand
	All   bool
	Force bool
}

type CleanupValues struct {
	PodmanCommand
	All    bool
	Latest bool
	Remove bool
}

type SystemPruneValues struct {
	PodmanCommand
	All    bool
	Force  bool
	Volume bool
}

type SystemRenumberValues struct {
	PodmanCommand
}

type SystemMigrateValues struct {
	PodmanCommand
	NewRuntime string
}

type SystemDfValues struct {
	PodmanCommand
	Verbose bool
	Format  string
}
