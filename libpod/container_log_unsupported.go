//+build !linux

package libpod

func (c *Container) readFromJournal(options *LogOptions, logChannel chan *LogLine) error {
	return ErrOSNotSupported
}
