// SPDX-License-Identifier: BSD-3-Clause
//go:build darwin && !cgo

package cpu

import "github.com/shirou/gopsutil/v4/internal/common"

func perCPUTimes() ([]TimesStat, error) {
	return []TimesStat{}, common.ErrNotImplementedError
}

func allCPUTimes() ([]TimesStat, error) {
	return []TimesStat{}, common.ErrNotImplementedError
}
