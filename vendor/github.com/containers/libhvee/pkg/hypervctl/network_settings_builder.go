//go:build windows

package hypervctl

type NetworkSettingsBuilder struct {
	systemSettings *SystemSettings
	err            error
}

type SyntheticEthernetPortSettingsBuilder struct {
	networkSettingsBuilder *NetworkSettingsBuilder
	portSettings           *NetworkAdapterParams
	err                    error
}

type EthernetPortAllocationSettingsBuilder struct {
	portSettingsBuilder *SyntheticEthernetPortSettingsBuilder
	allocSettings       *NetworkAdapterParams
	err                 error
}

func NewNetworkSettingsBuilder(systemSettings *SystemSettings) *NetworkSettingsBuilder {
	return &NetworkSettingsBuilder{systemSettings: systemSettings}
}

func (builder *NetworkSettingsBuilder) AddSyntheticEthernetPort(beforeAdd func(*NetworkAdapterParams)) *SyntheticEthernetPortSettingsBuilder {
	if builder.err != nil {
		return &SyntheticEthernetPortSettingsBuilder{networkSettingsBuilder: builder, err: builder.err}
	}

	portSettings := &NetworkAdapterParams{
		VMName: builder.systemSettings.Name,
	}
	if beforeAdd != nil {
		beforeAdd(portSettings)
	}

	return &SyntheticEthernetPortSettingsBuilder{
		networkSettingsBuilder: builder,
		portSettings:           portSettings,
		err:                    nil,
	}
}

func (builder *SyntheticEthernetPortSettingsBuilder) AddEthernetPortAllocation(switchName string) *EthernetPortAllocationSettingsBuilder {
	if builder.err != nil {
		return &EthernetPortAllocationSettingsBuilder{portSettingsBuilder: builder, err: builder.err}
	}

	allocSettings, err := builder.portSettings.DefineEthernetPortConnection(switchName)
	builder.setErr(err)

	return &EthernetPortAllocationSettingsBuilder{
		portSettingsBuilder: builder,
		allocSettings:       allocSettings,
		err:                 err,
	}
}

func (builder *SyntheticEthernetPortSettingsBuilder) Finish() *NetworkSettingsBuilder {
	return builder.networkSettingsBuilder
}

func (builder *EthernetPortAllocationSettingsBuilder) Finish() *SyntheticEthernetPortSettingsBuilder {
	return builder.portSettingsBuilder
}

func (builder *NetworkSettingsBuilder) setErr(err error) {
	builder.err = err
}

func (builder *SyntheticEthernetPortSettingsBuilder) setErr(err error) {
	builder.err = err
	builder.networkSettingsBuilder.setErr(err)
}

func (builder *EthernetPortAllocationSettingsBuilder) Get(s **NetworkAdapterParams) *EthernetPortAllocationSettingsBuilder {
	*s = builder.allocSettings
	return builder
}

func (builder *SyntheticEthernetPortSettingsBuilder) Get(s **NetworkAdapterParams) *SyntheticEthernetPortSettingsBuilder {
	*s = builder.portSettings
	return builder
}

func (builder *NetworkSettingsBuilder) Complete() error {
	return builder.err
}
