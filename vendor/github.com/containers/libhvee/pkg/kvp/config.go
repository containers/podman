//go:build linux

package kvp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrUnableToWriteToKVP is used when we are unable to write to the kernel
	// device for hyperv
	ErrUnableToWriteToKVP = errors.New("failed to write to hv_kvp")
	// ErrUnableToReadFromKVP is used when we are unable to read from the kernel
	// device for hyperv
	ErrUnableToReadFromKVP = errors.New("failed to read from hv_kvp")
	// ErrNoKeyValuePairsFound means we were unable to find key-value pairs as passed
	// from the hyperv host to this guest.
	ErrNoKeyValuePairsFound = errors.New("unable to find kvp keys")
	// ErrKeyNotFound means we could not find the key in information read
	ErrKeyNotFound = errors.New("unable to find key")
)

const (
	// Timeout amount of time in ms to poll the hyperv kernel device
	Timeout                   = 1000
	OpRegister1               = 100
	HvSOk                     = 0
	HvEFail                   = 0x80004005
	HvKvpExchangeMaxValueSize = 2048
	HvKvpExchangeMaxKeySize   = 512
	OpSet                     = 1
	// KernelDevice is the hyperv kernel device used for communicating key-value pairs
	// on hyperv between the host and guest
	KernelDevice = "/dev/vmbus/hv_kvp"
	// DefaultKVPPoolID is where Windows host write to for Linux VMs
	DefaultKVPPoolID               = 0
	DefaultKVPBaseName             = ".kvp_pool_"
	DefaultKVPFilePath             = "/var/lib/hyperv"
	DefaultKVPFileWritePermissions = 0644
)

type hvKvpExchgMsgValue struct {
	valueType uint32
	keySize   uint32
	valueSize uint32
	key       [HvKvpExchangeMaxKeySize]uint8
	value     [HvKvpExchangeMaxValueSize]uint8
}

type hvKvpMsgSet struct {
	data hvKvpExchgMsgValue
}

type hvKvpHdr struct {
	operation uint8
	pool      uint8
	pad       uint16
}

type hvKvpMsg struct {
	kvpHdr hvKvpHdr
	kvpSet hvKvpMsgSet
	// unused is needed to get to the same struct size as the C version.
	unused [4856]byte
}

type hvKvpMsgRet struct {
	// on 64-bit Linux, C int is 32 bits but Go int is 64 bits.  use
	// unsigned because error values are hex constants outside signed
	// integer range.
	error  uint32
	kvpSet hvKvpMsgSet
	// unused is needed to get to the same struct size as the C version.
	unused [4856]byte
}

type PoolID uint8

type ValuePair struct {
	Key   string
	Value string
}

type ValuePairs []ValuePair

func (vp ValuePairs) GetValueByKey(key string) (ValuePair, error) {
	for _, vp := range vp {
		if key == vp.Key {
			return vp, nil
		}
	}
	return ValuePair{}, ErrKeyNotFound
}

type KeyValuePair map[PoolID]ValuePairs

func (kv KeyValuePair) EncodePoolFile(poolID PoolID) (poolFile []byte) {
	poolEntries, exists := kv[poolID]
	if !exists {
		return
	}
	for _, entry := range poolEntries {
		// These have to be padded with nulls
		emptyKey := make([]byte, HvKvpExchangeMaxKeySize)
		emptyVal := make([]byte, HvKvpExchangeMaxValueSize)
		_ = copy(emptyKey, entry.Key)
		_ = copy(emptyVal, entry.Value)
		poolFile = append(poolFile, emptyKey...)
		poolFile = append(poolFile, emptyVal...)
	}
	return
}

func (kv KeyValuePair) append(poolID PoolID, key, value string) {
	vps, exists := kv[poolID]
	vp := ValuePair{
		Key:   key,
		Value: value,
	}
	if !exists {
		kv[poolID] = ValuePairs{vp}
		return
	}
	kv[poolID] = append(vps, vp)
}

func (kv KeyValuePair) WriteToFS(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	for poolID := range kv {
		fqWritePath := filepath.Join(path, fmt.Sprintf("%s%d", DefaultKVPBaseName, poolID))
		if len(kv[poolID]) < 1 {
			// need to set permissions so ...
			if err := os.WriteFile(fqWritePath, []byte{}, DefaultKVPFileWritePermissions); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(fqWritePath, kv.EncodePoolFile(poolID), DefaultKVPFileWritePermissions); err != nil {
			return err
		}
	}
	return nil
}
