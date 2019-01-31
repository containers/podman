package cliconfig

// GlobalIsSet is a compatibility method for urfave
func (p *PodmanCommand) GlobalIsSet(opt string) bool {
	flag := p.PersistentFlags().Lookup(opt)
	if flag == nil {
		return false
	}
	return flag.Changed
}

// IsSet is a compatibility method for urfave
func (p *PodmanCommand) IsSet(opt string) bool {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return false
	}
	return flag.Changed
}

// Bool is a compatibility method for urfave
func (p *PodmanCommand) Bool(opt string) bool {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return false
	}
	val, _ := p.Flags().GetBool(opt)
	return val
}

// String is a compatibility method for urfave
func (p *PodmanCommand) String(opt string) string {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return ""
	}
	val, _ := p.Flags().GetString(opt)
	return val
}

// StringArray is a compatibility method for urfave
func (p *PodmanCommand) StringArray(opt string) []string {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return []string{}
	}
	val, _ := p.Flags().GetStringArray(opt)
	return val
}

// StringSlice is a compatibility method for urfave
func (p *PodmanCommand) StringSlice(opt string) []string {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return []string{}
	}
	val, _ := p.Flags().GetStringSlice(opt)
	return val
}

// Int is a compatibility method for urfave
func (p *PodmanCommand) Int(opt string) int {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return 0
	}
	val, _ := p.Flags().GetInt(opt)
	return val
}

// Unt is a compatibility method for urfave
func (p *PodmanCommand) Uint(opt string) uint {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return 0
	}
	val, _ := p.Flags().GetUint(opt)
	return val
}

// Int64 is a compatibility method for urfave
func (p *PodmanCommand) Int64(opt string) int64 {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return 0
	}
	val, _ := p.Flags().GetInt64(opt)
	return val
}

// Unt64 is a compatibility method for urfave
func (p *PodmanCommand) Uint64(opt string) uint64 {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return 0
	}
	val, _ := p.Flags().GetUint64(opt)
	return val
}

// Float64 is a compatibility method for urfave
func (p *PodmanCommand) Float64(opt string) float64 {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		return 0
	}
	val, _ := p.Flags().GetFloat64(opt)
	return val
}
