// SPDX-License-Identifier: BSD-3-Clause
//go:build !aix && !darwin && !linux && !freebsd && !openbsd && !windows && !solaris

package net

import (
	"context"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func IOCountersWithContext(ctx context.Context, pernic bool) ([]IOCountersStat, error) {
	return []IOCountersStat{}, common.ErrNotImplementedError
}

func IOCountersByFileWithContext(ctx context.Context, pernic bool, filename string) ([]IOCountersStat, error) {
	return IOCountersWithContext(ctx, pernic)
}

func FilterCountersWithContext(ctx context.Context) ([]FilterStat, error) {
	return nil, common.ErrNotImplementedError
}

func ConntrackStatsWithContext(ctx context.Context, percpu bool) ([]ConntrackStat, error) {
	return nil, common.ErrNotImplementedError
}

func ProtoCountersWithContext(ctx context.Context, protocols []string) ([]ProtoCountersStat, error) {
	return nil, common.ErrNotImplementedError
}

// Deprecated: use process.PidsWithContext instead
func PidsWithContext(ctx context.Context) ([]int32, error) {
	return nil, common.ErrNotImplementedError
}

func ConnectionsWithContext(ctx context.Context, kind string) ([]ConnectionStat, error) {
	return []ConnectionStat{}, common.ErrNotImplementedError
}

func ConnectionsMaxWithContext(ctx context.Context, kind string, maxConn int) ([]ConnectionStat, error) {
	return ConnectionsPidMaxWithContext(ctx, kind, 0, maxConn)
}

func ConnectionsWithoutUidsWithContext(ctx context.Context, kind string) ([]ConnectionStat, error) {
	return ConnectionsMaxWithoutUidsWithContext(ctx, kind, 0)
}

func ConnectionsMaxWithoutUidsWithContext(ctx context.Context, kind string, maxConn int) ([]ConnectionStat, error) {
	return ConnectionsPidMaxWithoutUidsWithContext(ctx, kind, 0, maxConn)
}

func ConnectionsPidWithoutUidsWithContext(ctx context.Context, kind string, pid int32) ([]ConnectionStat, error) {
	return ConnectionsPidMaxWithoutUidsWithContext(ctx, kind, pid, 0)
}

func ConnectionsPidWithContext(ctx context.Context, kind string, pid int32) ([]ConnectionStat, error) {
	return ConnectionsPidMaxWithContext(ctx, kind, pid, 0)
}

func ConnectionsPidMaxWithContext(ctx context.Context, kind string, pid int32, maxConn int) ([]ConnectionStat, error) {
	return connectionsPidMaxWithoutUidsWithContext(ctx, kind, pid, maxConn, false)
}

func ConnectionsPidMaxWithoutUidsWithContext(ctx context.Context, kind string, pid int32, maxConn int) ([]ConnectionStat, error) {
	return connectionsPidMaxWithoutUidsWithContext(ctx, kind, pid, maxConn, true)
}

func connectionsPidMaxWithoutUidsWithContext(_ context.Context, _ string, _ int32, _ int, _ bool) ([]ConnectionStat, error) {
	return []ConnectionStat{}, common.ErrNotImplementedError
}
