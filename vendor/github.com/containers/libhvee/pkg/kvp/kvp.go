//go:build linux

package kvp

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// readKvpData reads all key-value pairs from the hyperv kernel device and creates
// a map representation of them
func readKvpData() (KeyValuePair, error) {
	ret := make(KeyValuePair)
	for i := 0; i < 5; i++ {
		// We need to seed the poolids
		ret[PoolID(i)] = ValuePairs{}
	}
	kvp, err := unix.Open(KernelDevice, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(kvp)

	var (
		hvMsg    hvKvpMsg
		hvMsgRet hvKvpMsgRet
	)

	const sizeOf = int(unsafe.Sizeof(hvMsg))

	var (
		asByteSlice    = (*(*[sizeOf]byte)(unsafe.Pointer(&hvMsg)))[:]
		retAsByteSlice = (*(*[sizeOf]byte)(unsafe.Pointer(&hvMsgRet)))[:]
	)

	hvMsg.kvpHdr.operation = OpRegister1

	l, err := unix.Write(kvp, asByteSlice)
	if err != nil {
		return nil, err
	}
	if l != sizeOf {
		return nil, ErrUnableToWriteToKVP
	}

next:
	for {
		var pfd unix.PollFd
		pfd.Fd = int32(kvp)
		pfd.Events = unix.POLLIN
		pfd.Revents = 0

		howMany, err := unix.Poll([]unix.PollFd{pfd}, Timeout)
		if err != nil {
			// loop on retryable errors
			if err == unix.EINTR {
				continue
			}
			return nil, err
		}

		if howMany == 0 {
			return ret, nil
		}

		l, err := unix.Read(kvp, asByteSlice)
		if err != nil {
			// loop on retryable errors
			if err == unix.EAGAIN || err == unix.EINTR || err == unix.EWOULDBLOCK {
				continue
			}
			return nil, err
		}
		if l != sizeOf {
			return nil, ErrUnableToReadFromKVP
		}

		switch hvMsg.kvpHdr.operation {
		case OpRegister1:
			continue next
		case OpSet:
			// on the next two variables, we are cutting the last byte because otherwise
			// it is padded and key lookups fail
			key := hvMsg.kvpSet.data.key[:hvMsg.kvpSet.data.keySize-1]
			value := hvMsg.kvpSet.data.value[:hvMsg.kvpSet.data.valueSize-1]

			poolID := PoolID(hvMsg.kvpHdr.pool)
			ret.append(poolID, string(key), string(value))

			hvMsgRet.error = HvSOk
		default:
			hvMsgRet.error = HvEFail
		}

		l, err = unix.Write(kvp, retAsByteSlice)
		if err != nil {
			return nil, err
		}
		if l != sizeOf {
			return nil, ErrUnableToWriteToKVP
		}
	}
}

// GetKeyValuePairs reads the key value pairs from the wmi hyperv kernel device
// and returns them in map form.  the map value is a ValuePair which contains
// the value string and the poolid
func GetKeyValuePairs() (KeyValuePair, error) {
	return readKvpData()
}

// GetSplitKeyValues reassembles split KVPs from a key prefix and pool_id and
// returns the assembled split value.
func (kv KeyValuePair) GetSplitKeyValues(key string, pool PoolID) (string, error) {
	var (
		parts   []string
		counter = 0
	)

	for {
		wantKey := fmt.Sprintf("%s%d", key, counter)
		entries, exists := kv[pool]
		if !exists {
			// No entries for the pool
			break
		}
		entry, err := entries.GetValueByKey(wantKey)
		if err != nil {
			if errors.Is(err, ErrKeyNotFound) {
				break
			}

			return "", err
		}
		parts = append(parts, entry.Value)
		counter++
	}
	if len(parts) < 1 {
		return "", ErrNoKeyValuePairsFound
	}
	return strings.Join(parts, ""), nil
}
