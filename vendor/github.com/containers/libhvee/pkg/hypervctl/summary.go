//go:build windows

package hypervctl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libhvee/pkg/powershell"
)

// PowerShellVM represents the structure returned by Get-VM | ConvertTo-Json
type PowerShellVM struct {
	ConfigurationLocation               string                           `json:"ConfigurationLocation"`
	GuestStatePath                      string                           `json:"GuestStatePath"`
	SmartPagingFileInUse                bool                             `json:"SmartPagingFileInUse"`
	SmartPagingFilePath                 string                           `json:"SmartPagingFilePath"`
	SnapshotFileLocation                string                           `json:"SnapshotFileLocation"`
	AutomaticStartAction                int                              `json:"AutomaticStartAction"`
	AutomaticStartDelay                 int                              `json:"AutomaticStartDelay"`
	AutomaticStopAction                 int                              `json:"AutomaticStopAction"`
	AutomaticCriticalErrorAction        int                              `json:"AutomaticCriticalErrorAction"`
	AutomaticCriticalErrorActionTimeout int                              `json:"AutomaticCriticalErrorActionTimeout"`
	AutomaticCheckpointsEnabled         bool                             `json:"AutomaticCheckpointsEnabled"`
	CPUUsage                            int                              `json:"CPUUsage"`
	MemoryAssigned                      int64                            `json:"MemoryAssigned"`
	MemoryDemand                        int64                            `json:"MemoryDemand"`
	MemoryStatus                        string                           `json:"MemoryStatus"`
	NumaAligned                         *bool                            `json:"NumaAligned"`
	NumaNodesCount                      int                              `json:"NumaNodesCount"`
	NumaSocketCount                     int                              `json:"NumaSocketCount"`
	Heartbeat                           *int                             `json:"Heartbeat"`
	IntegrationServicesState            string                           `json:"IntegrationServicesState"`
	IntegrationServicesVersion          PowerShellVersion                `json:"IntegrationServicesVersion"`
	Uptime                              PowerShellTimeSpan               `json:"Uptime"`
	OperationalStatus                   []int                            `json:"OperationalStatus"`
	PrimaryOperationalStatus            int                              `json:"PrimaryOperationalStatus"`
	SecondaryOperationalStatus          *int                             `json:"SecondaryOperationalStatus"`
	StatusDescriptions                  []string                         `json:"StatusDescriptions"`
	PrimaryStatusDescription            string                           `json:"PrimaryStatusDescription"`
	SecondaryStatusDescription          *string                          `json:"SecondaryStatusDescription"`
	Status                              string                           `json:"Status"`
	ReplicationHealth                   int                              `json:"ReplicationHealth"`
	ReplicationMode                     int                              `json:"ReplicationMode"`
	ReplicationState                    int                              `json:"ReplicationState"`
	ResourceMeteringEnabled             bool                             `json:"ResourceMeteringEnabled"`
	CheckpointType                      int                              `json:"CheckpointType"`
	EnhancedSessionTransportType        int                              `json:"EnhancedSessionTransportType"`
	Groups                              []interface{}                    `json:"Groups"`
	Version                             string                           `json:"Version"`
	VirtualMachineType                  int                              `json:"VirtualMachineType"`
	VirtualMachineSubType               int                              `json:"VirtualMachineSubType"`
	GuestStateIsolationType             int                              `json:"GuestStateIsolationType"`
	Notes                               string                           `json:"Notes"`
	State                               EnabledState                     `json:"State"`
	ComPorts                            []*PowerShellComPort             `json:"ComPorts,omitempty"`
	ComPort1                            *PowerShellComPort               `json:"ComPort1,omitempty"`
	ComPort2                            *PowerShellComPort               `json:"ComPort2,omitempty"`
	DVDDrives                           []*PowerShellDvdDrive            `json:"DVDDrives"`
	FibreChannelHostBusAdapters         []interface{}                    `json:"FibreChannelHostBusAdapters"`
	FloppyDrive                         *PowerShellFloppyDrive           `json:"FloppyDrive,omitempty"`
	HardDrives                          []*HardDiskDrive                 `json:"HardDrives"`
	RemoteFxAdapter                     *interface{}                     `json:"RemoteFxAdapter,omitempty"`
	DynamicMemoryEnabled                bool                             `json:"DynamicMemoryEnabled"`
	MemoryMaximum                       int64                            `json:"MemoryMaximum"`
	MemoryMinimum                       int64                            `json:"MemoryMinimum"`
	MemoryStartup                       int64                            `json:"MemoryStartup"`
	ProcessorCount                      int                              `json:"ProcessorCount"`
	BatteryPassthroughEnabled           bool                             `json:"BatteryPassthroughEnabled"`
	Generation                          int                              `json:"Generation"`
	IsClustered                         bool                             `json:"IsClustered"`
	ParentSnapshotId                    *string                          `json:"ParentSnapshotId"`
	ParentSnapshotName                  *string                          `json:"ParentSnapshotName"`
	Path                                string                           `json:"Path"`
	SizeOfSystemFiles                   int64                            `json:"SizeOfSystemFiles"`
	GuestControlledCacheTypes           bool                             `json:"GuestControlledCacheTypes"`
	LowMemoryMappedIoSpace              int64                            `json:"LowMemoryMappedIoSpace"`
	HighMemoryMappedIoSpace             int64                            `json:"HighMemoryMappedIoSpace"`
	HighMemoryMappedIoBaseAddress       int64                            `json:"HighMemoryMappedIoBaseAddress"`
	LockOnDisconnect                    int                              `json:"LockOnDisconnect"`
	CreationTime                        string                           `json:"CreationTime"` // .NET DateTime format like "/Date(1756216458313)/"
	Id                                  string                           `json:"Id"`
	Name                                string                           `json:"Name"`
	NetworkAdapters                     []*PowerShellNetworkAdapter      `json:"NetworkAdapters,omitempty"`
	ComputerName                        string                           `json:"ComputerName"`
	IsDeleted                           bool                             `json:"IsDeleted"`
	ParentCheckpointId                  *string                          `json:"ParentCheckpointId"`
	ParentCheckpointName                *string                          `json:"ParentCheckpointName"`
	VMName                              string                           `json:"VMName"`
	VMId                                string                           `json:"VMId"`
	CheckpointFileLocation              string                           `json:"CheckpointFileLocation"`
}

type PowerShellVersion struct {
	Major         int `json:"Major"`
	Minor         int `json:"Minor"`
	Build         int `json:"Build"`
	Revision      int `json:"Revision"`
	MajorRevision int `json:"MajorRevision"`
	MinorRevision int `json:"MinorRevision"`
}

type PowerShellTimeSpan struct {
	Ticks             int64   `json:"Ticks"`
	Days              int     `json:"Days"`
	Hours             int     `json:"Hours"`
	Milliseconds      int     `json:"Milliseconds"`
	Minutes           int     `json:"Minutes"`
	Seconds           int     `json:"Seconds"`
	TotalDays         float64 `json:"TotalDays"`
	TotalHours        float64 `json:"TotalHours"`
	TotalMilliseconds float64 `json:"TotalMilliseconds"`
	TotalMinutes      float64 `json:"TotalMinutes"`
	TotalSeconds      float64 `json:"TotalSeconds"`
}

type PowerShellComPort struct {
	Path           string `json:"Path"`
	DebuggerMode   int    `json:"DebuggerMode"`
	Name           string `json:"Name"`
	Id             string `json:"Id"`
	VMId           string `json:"VMId"`
	VMName         string `json:"VMName"`
	VMSnapshotId   string `json:"VMSnapshotId"`
	VMSnapshotName string `json:"VMSnapshotName"`
	ComputerName   string `json:"ComputerName"`
	IsDeleted      bool   `json:"IsDeleted"`
}

type PowerShellFloppyDrive struct {
	Path           *string `json:"Path"`
	PoolName       *string `json:"PoolName"`
	Name           string  `json:"Name"`
	Id             string  `json:"Id"`
	VMId           string  `json:"VMId"`
	VMName         string  `json:"VMName"`
	VMSnapshotId   string  `json:"VMSnapshotId"`
	VMSnapshotName string  `json:"VMSnapshotName"`
	ComputerName   string  `json:"ComputerName"`
	IsDeleted      bool    `json:"IsDeleted"`
}

type HardDiskDrive struct {
	Path                          string `json:"Path"`
	DiskNumber                    *int   `json:"DiskNumber"`
	MaximumIOPS                   int    `json:"MaximumIOPS"`
	MinimumIOPS                   int    `json:"MinimumIOPS"`
	QoSPolicyID                   string `json:"QoSPolicyID"`
	SupportPersistentReservations bool   `json:"SupportPersistentReservations"`
	WriteHardeningMethod          int    `json:"WriteHardeningMethod"`
	ControllerLocation            int    `json:"ControllerLocation"`
	ControllerNumber              int    `json:"ControllerNumber"`
	ControllerType                int    `json:"ControllerType"`
	Name                          string `json:"Name"`
	PoolName                      string `json:"PoolName"`
	Id                            string `json:"Id"`
	VMId                          string `json:"VMId"`
	VMName                        string `json:"VMName"`
	VMSnapshotId                  string `json:"VMSnapshotId"`
	VMSnapshotName                string `json:"VMSnapshotName"`
	ComputerName                  string `json:"ComputerName"`
	IsDeleted                     bool   `json:"IsDeleted"`
}

type PowerShellDvdDrive struct {
	DvdMediaType       int    `json:"DvdMediaType"`
	Path               string `json:"Path"`
	ControllerLocation int    `json:"ControllerLocation"`
	ControllerNumber   int    `json:"ControllerNumber"`
	ControllerType     int    `json:"ControllerType"`
	Name               string `json:"Name"`
	PoolName           string `json:"PoolName"`
	Id                 string `json:"Id"`
	VMId               string `json:"VMId"`
	VMName             string `json:"VMName"`
	VMSnapshotId       string `json:"VMSnapshotId"`
	VMSnapshotName     string `json:"VMSnapshotName"`
	ComputerName       string `json:"ComputerName"`
	IsDeleted          bool   `json:"IsDeleted"`
}

type PowerShellNetworkAdapter struct {
	ClusterMonitored                        bool          `json:"ClusterMonitored"`
	MacAddress                              string        `json:"MacAddress"`
	MediaType                               int           `json:"MediaType"`
	DynamicMacAddressEnabled                bool          `json:"DynamicMacAddressEnabled"`
	InterruptModeration                     bool          `json:"InterruptModeration"`
	AllowPacketDirect                       bool          `json:"AllowPacketDirect"`
	VirtualSystemIdentifiers                StringOrArray `json:"VirtualSystemIdentifiers"`
	NumaAwarePlacement                      bool          `json:"NumaAwarePlacement"`
	IsLegacy                                bool          `json:"IsLegacy"`
	IsSynthetic                             bool          `json:"IsSynthetic"`
	IPAddresses                             StringOrArray `json:"IPAddresses"`
	DeviceNaming                            int           `json:"DeviceNaming"`
	IovWeight                               int           `json:"IovWeight"`
	IovQueuePairsRequested                  int           `json:"IovQueuePairsRequested"`
	IovInterruptModeration                  int           `json:"IovInterruptModeration"`
	PacketDirectNumProcs                    int           `json:"PacketDirectNumProcs"`
	PacketDirectModerationCount             int           `json:"PacketDirectModerationCount"`
	PacketDirectModerationInterval          int           `json:"PacketDirectModerationInterval"`
	IovQueuePairsAssigned                   int           `json:"IovQueuePairsAssigned"`
	IovUsage                                int           `json:"IovUsage"`
	VirtualFunction                         *interface{}  `json:"VirtualFunction"`
	MandatoryFeatureId                      StringOrArray `json:"MandatoryFeatureId"`
	MandatoryFeatureName                    StringOrArray `json:"MandatoryFeatureName"`
	PoolName                                string        `json:"PoolName"`
	Connected                               bool          `json:"Connected"`
	SwitchName                              string        `json:"SwitchName"`
	AdapterId                               string        `json:"AdapterId"`
	TestReplicaPoolName                     string        `json:"TestReplicaPoolName"`
	TestReplicaSwitchName                   string        `json:"TestReplicaSwitchName"`
	StatusDescription                       string        `json:"StatusDescription"`
	Status                                  string        `json:"Status"`
	IsManagementOs                          bool          `json:"IsManagementOs"`
	IsExternalAdapter                       bool          `json:"IsExternalAdapter"`
	Id                                      string        `json:"Id"`
	SwitchId                                string        `json:"SwitchId"`
	AclList                                 StringOrArray `json:"AclList"`
	ExtendedAclList                         StringOrArray `json:"ExtendedAclList"`
	IsolationSetting                        *interface{}  `json:"IsolationSetting"`
	RoutingDomainList                       StringOrArray `json:"RoutingDomainList"`
	VlanSetting                             *interface{}  `json:"VlanSetting"`
	BandwidthSetting                        *interface{}  `json:"BandwidthSetting"`
	CurrentIsolationMode                    int           `json:"CurrentIsolationMode"`
	MacAddressSpoofing                      int           `json:"MacAddressSpoofing"`
	DhcpGuard                               int           `json:"DhcpGuard"`
	RouterGuard                             int           `json:"RouterGuard"`
	PortMirroringMode                       int           `json:"PortMirroringMode"`
	IeeePriorityTag                         int           `json:"IeeePriorityTag"`
	VirtualSubnetId                         int           `json:"VirtualSubnetId"`
	DynamicIPAddressLimit                   int           `json:"DynamicIPAddressLimit"`
	StormLimit                              int           `json:"StormLimit"`
	AllowTeaming                            int           `json:"AllowTeaming"`
	FixSpeed10G                             int           `json:"FixSpeed10G"`
	VMQWeight                               int           `json:"VMQWeight"`
	IPsecOffloadMaxSA                       int           `json:"IPsecOffloadMaxSA"`
	VrssEnabled                             bool          `json:"VrssEnabled"`
	VrssEnabledRequested                    bool          `json:"VrssEnabledRequested"`
	VmmqEnabled                             bool          `json:"VmmqEnabled"`
	VmmqEnabledRequested                    bool          `json:"VmmqEnabledRequested"`
	VrssMaxQueuePairs                       int           `json:"VrssMaxQueuePairs"`
	VrssMaxQueuePairsRequested              int           `json:"VrssMaxQueuePairsRequested"`
	VrssMinQueuePairs                       int           `json:"VrssMinQueuePairs"`
	VrssMinQueuePairsRequested              int           `json:"VrssMinQueuePairsRequested"`
	VrssQueueSchedulingMode                 int           `json:"VrssQueueSchedulingMode"`
	VrssQueueSchedulingModeRequested        int           `json:"VrssQueueSchedulingModeRequested"`
	VrssExcludePrimaryProcessor             bool          `json:"VrssExcludePrimaryProcessor"`
	VrssExcludePrimaryProcessorRequested    bool          `json:"VrssExcludePrimaryProcessorRequested"`
	VrssIndependentHostSpreading            bool          `json:"VrssIndependentHostSpreading"`
	VrssIndependentHostSpreadingRequested   bool          `json:"VrssIndependentHostSpreadingRequested"`
	VrssVmbusChannelAffinityPolicy          int           `json:"VrssVmbusChannelAffinityPolicy"`
	VrssVmbusChannelAffinityPolicyRequested int           `json:"VrssVmbusChannelAffinityPolicyRequested"`
	RscEnabled                              bool          `json:"RscEnabled"`
	RscEnabledRequested                     bool          `json:"RscEnabledRequested"`
	VmqUsage                                int           `json:"VmqUsage"`
	IPsecOffloadSAUsage                     int           `json:"IPsecOffloadSAUsage"`
	VFDataPathActive                        bool          `json:"VFDataPathActive"`
	VMQueue                                 *interface{}  `json:"VMQueue"`
	BandwidthPercentage                     int           `json:"BandwidthPercentage"`
	IsTemplate                              bool          `json:"IsTemplate"`
	Name                                    string        `json:"Name"`
	VMId                                    string        `json:"VMId"`
	VMName                                  string        `json:"VMName"`
	VMSnapshotId                            string        `json:"VMSnapshotId"`
	VMSnapshotName                          string        `json:"VMSnapshotName"`
	ComputerName                            string        `json:"ComputerName"`
	IsDeleted                               bool          `json:"IsDeleted"`
}

type PowerShellIntegrationService struct {
	Enabled                    bool     `json:"Enabled"`
	OperationalStatus          []int    `json:"OperationalStatus,omitempty"`
	PrimaryOperationalStatus   int      `json:"PrimaryOperationalStatus"`
	PrimaryStatusDescription   string   `json:"PrimaryStatusDescription"`
	SecondaryOperationalStatus int      `json:"SecondaryOperationalStatus"`
	SecondaryStatusDescription string   `json:"SecondaryStatusDescription"`
	StatusDescription          []string `json:"StatusDescription,omitempty"`
	Name                       string   `json:"Name"`
	Id                         string   `json:"Id"`
	VMId                       string   `json:"VMId"`
	VMName                     string   `json:"VMName"`
	VMSnapshotId               string   `json:"VMSnapshotId"`
	VMSnapshotName             string   `json:"VMSnapshotName"`
	ComputerName               string   `json:"ComputerName"`
	IsDeleted                  bool     `json:"IsDeleted"`
}

type PowerShellVMIntegrationService struct {
	Enabled                    bool          `json:"Enabled"`
	OperationalStatus          StringOrArray `json:"OperationalStatus,omitempty"`
	PrimaryOperationalStatus   int           `json:"PrimaryOperationalStatus"`
	PrimaryStatusDescription   string        `json:"PrimaryStatusDescription"`
	SecondaryOperationalStatus int           `json:"SecondaryOperationalStatus"`
	SecondaryStatusDescription string        `json:"SecondaryStatusDescription"`
	StatusDescription          StringOrArray `json:"StatusDescription,omitempty"`
	Name                       string        `json:"Name"`
	Id                         string        `json:"Id"`
	VMId                       string        `json:"VMId"`
	VMName                     string        `json:"VMName"`
	VMSnapshotId               string        `json:"VMSnapshotId"`
	VMSnapshotName             string        `json:"VMSnapshotName"`
	ComputerName               string        `json:"ComputerName"`
	IsDeleted                  bool          `json:"IsDeleted"`
}

type PowerShellCimSession struct {
	ComputerName *string `json:"ComputerName"`
	InstanceId   string  `json:"InstanceId"`
}

// parseDotNetDateTime parses .NET DateTime format like "/Date(1756216458313)/"
func parseDotNetDateTime(dateStr string) (time.Time, error) {
	// Regular expression to match /Date(timestamp)/
	re := regexp.MustCompile(`^/Date\((\d+)\)/$`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) != 2 {
		return time.Time{}, nil // Return zero time for invalid format
	}

	timestamp, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	// .NET DateTime ticks are in milliseconds since Unix epoch
	return time.Unix(timestamp/1000, (timestamp%1000)*1000000), nil
}

// GetVMsFromPowerShell executes Get-VM | ConvertTo-Json and parses the result
func GetVMsFromPowerShell() ([]PowerShellVM, error) {
	stdout, stderr, err := powershell.Execute("Get-VM | ConvertTo-Json -Depth 5 -Compress")
	if err != nil {
		return nil, NewPSError(stderr)
	}

	// Remove any BOM or extra whitespace
	stdout = strings.TrimSpace(stdout)

	// Handle single object response
	if !strings.HasPrefix(stdout, "[") {
		stdout = "[" + stdout + "]"
	}

	var vms []PowerShellVM
	if err := json.Unmarshal([]byte(stdout), &vms); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	return vms, nil
}

// GetVMFromPowerShell executes Get-VM -Name <name> | ConvertTo-Json and parses the result
func GetVMFromPowerShell(name string) (*PowerShellVM, error) {
	stdout, stderr, err := powershell.Execute("Get-VM", "-Name", name, "|", "ConvertTo-Json", "-Depth", "5", "-Compress")
	if err != nil {
		return nil, NewPSError(stderr)
	}

	// Remove any BOM or extra whitespace
	stdout = strings.TrimSpace(stdout)

	var vm PowerShellVM
	if err := json.Unmarshal([]byte(stdout), &vm); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	return &vm, nil
}

// ConvertToVirtualMachine converts a PowerShellVM to a VirtualMachine instance
func (psvm *PowerShellVM) ConvertToVirtualMachine(vmm *VirtualMachineManager) *VirtualMachine {

	return &VirtualMachine{
		PowerShellVM: *psvm,
		vmm:          vmm,
	}
}

func (psvm *PowerShellVM) ConvertToCreationTime() time.Time {
	creationTime, err := parseDotNetDateTime(psvm.CreationTime)
	if err != nil {
		return time.Time{}
	}
	return creationTime
}
