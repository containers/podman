//go:build linux

package loopback

type loopInfo64 struct {
	loDevice         uint64 /* ioctl r/o */
	loInode          uint64 /* ioctl r/o */
	loRdevice        uint64 /* ioctl r/o */
	loOffset         uint64
	loSizelimit      uint64 /* bytes, 0 == max available */
	loNumber         uint32 /* ioctl r/o */
	loEncryptType    uint32
	loEncryptKeySize uint32 /* ioctl w/o */
	loFlags          uint32 /* ioctl r/o */
	loFileName       [LoNameSize]uint8
	loCryptName      [LoNameSize]uint8
	loEncryptKey     [LoKeySize]uint8 /* ioctl w/o */
	loInit           [2]uint64
}

// IOCTL consts
const (
	LoopSetFd       = 0x4C00
	LoopCtlGetFree  = 0x4C82
	LoopGetStatus64 = 0x4C05
	LoopSetStatus64 = 0x4C04
	LoopClrFd       = 0x4C01
	LoopSetCapacity = 0x4C07
)

// LOOP consts.
const (
	LoFlagsAutoClear = 0x4C07
	LoFlagsReadOnly  = 1
	LoFlagsPartScan  = 8
	LoKeySize        = 32
	LoNameSize       = 64
)
