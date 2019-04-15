package cliconfig

import (
	"github.com/sirupsen/logrus"
)

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
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return false
	}
	val, err := p.Flags().GetBool(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// String is a compatibility method for urfave
func (p *PodmanCommand) String(opt string) string {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return ""
	}
	val, err := p.Flags().GetString(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// StringArray is a compatibility method for urfave
func (p *PodmanCommand) StringArray(opt string) []string {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return []string{}
	}
	val, err := p.Flags().GetStringArray(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// StringSlice is a compatibility method for urfave
func (p *PodmanCommand) StringSlice(opt string) []string {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return []string{}
	}
	val, err := p.Flags().GetStringSlice(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// Int is a compatibility method for urfave
func (p *PodmanCommand) Int(opt string) int {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return 0
	}
	val, err := p.Flags().GetInt(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// Unt is a compatibility method for urfave
func (p *PodmanCommand) Uint(opt string) uint {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return 0
	}
	val, err := p.Flags().GetUint(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// Int64 is a compatibility method for urfave
func (p *PodmanCommand) Int64(opt string) int64 {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return 0
	}
	val, err := p.Flags().GetInt64(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// Unt64 is a compatibility method for urfave
func (p *PodmanCommand) Uint64(opt string) uint64 {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return 0
	}
	val, err := p.Flags().GetUint64(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}

// Float64 is a compatibility method for urfave
func (p *PodmanCommand) Float64(opt string) float64 {
	flag := p.Flags().Lookup(opt)
	if flag == nil {
		if !p.Remote {
			logrus.Errorf("Could not find flag %s", opt)
		}
		return 0
	}
	val, err := p.Flags().GetFloat64(opt)
	if err != nil {
		logrus.Errorf("Error getting flag %s: %v", opt, err)
	}
	return val
}
