package cliconfig

import (
	"github.com/spf13/cobra"
)

type PodmanCommand struct {
	*cobra.Command
	InputArgs   []string
	GlobalFlags MainFlags
}

type MainFlags struct {
	CGroupManager     string
	CniConfigDir      string
	ConmonPath        string
	DefaultMountsFile string
	HooksDir          []string
	MaxWorks          int
	Namespace         string
	Root              string
	Runroot           string
	Runtime           string
	StorageDriver     string
	StorageOpts       []string
	Syslog            bool

	Config     string
	CpuProfile string
	LogLevel   string
	TmpDir     string
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

type TagValues struct {
	PodmanCommand
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
}

type CommitValues struct {
	PodmanCommand
	Change  []string
	Format  string
	Message string
	Author  string
	Pause   bool
	Quiet   bool
}

type ContainersPrune struct {
	PodmanCommand
}

type DiffValues struct {
	PodmanCommand
	Archive bool
	Format  string
}

type ExecValues struct {
	PodmanCommand
	Env          []string
	Privileged   bool
	Interfactive bool
	Tty          bool
	User         string
	Latest       bool
	Workdir      string
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
	Service bool
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

type PauseValues struct {
	PodmanCommand
	All bool
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
}

type PullValues struct {
	PodmanCommand
	Authfile        string
	CertDir         string
	Creds           string
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
}

type RmValues struct {
	PodmanCommand
	All     bool
	Force   bool
	Latest  bool
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
	Display         bool
	CertDir         string
	Creds           string
	Name            string
	Opt1            string
	Opt2            string
	Opt3            string
	Quiet           bool
	Pull            bool
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

type SignValues struct {
	PodmanCommand
	Directory string
	SignBy    string
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
}

type SystemPruneValues struct {
	PodmanCommand
	All    bool
	Force  bool
	Volume bool
}
