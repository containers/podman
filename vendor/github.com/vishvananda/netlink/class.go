package netlink

import (
	"fmt"
)

type Class interface {
	Attrs() *ClassAttrs
	Type() string
}

// Generic networking statistics for netlink users.
// This file contains "gnet_" prefixed structs and relevant functions.
// See Documentation/networking/getn_stats.txt in Linux source code for more details.

// Ref: struct gnet_stats_basic { ... }
type GnetStatsBasic struct {
	Bytes   uint64 // number of seen bytes
	Packets uint32 // number of seen packets
}

// Ref: struct gnet_stats_rate_est { ... }
type GnetStatsRateEst struct {
	Bps uint32 // current byte rate
	Pps uint32 // current packet rate
}

// Ref: struct gnet_stats_rate_est64 { ... }
type GnetStatsRateEst64 struct {
	Bps uint64 // current byte rate
	Pps uint64 // current packet rate
}

// Ref: struct gnet_stats_queue { ... }
type GnetStatsQueue struct {
	Qlen       uint32 // queue length
	Backlog    uint32 // backlog size of queue
	Drops      uint32 // number of dropped packets
	Requeues   uint32 // number of requues
	Overlimits uint32 // number of enqueues over the limit
}

// Statistics representaion based on generic networking statisticsfor netlink.
// See Documentation/networking/gen_stats.txt in Linux source code for more details.
type ClassStatistics struct {
	Basic   *GnetStatsBasic
	Queue   *GnetStatsQueue
	RateEst *GnetStatsRateEst
}

// Construct a ClassStatistics struct which fields are all initialized by 0.
func NewClassStatistics() *ClassStatistics {
	return &ClassStatistics{
		Basic:   &GnetStatsBasic{},
		Queue:   &GnetStatsQueue{},
		RateEst: &GnetStatsRateEst{},
	}
}

// ClassAttrs represents a netlink class. A filter is associated with a link,
// has a handle and a parent. The root filter of a device should have a
// parent == HANDLE_ROOT.
type ClassAttrs struct {
	LinkIndex  int
	Handle     uint32
	Parent     uint32
	Leaf       uint32
	Statistics *ClassStatistics
}

func (q ClassAttrs) String() string {
	return fmt.Sprintf("{LinkIndex: %d, Handle: %s, Parent: %s, Leaf: %d}", q.LinkIndex, HandleStr(q.Handle), HandleStr(q.Parent), q.Leaf)
}

type HtbClassAttrs struct {
	// TODO handle all attributes
	Rate    uint64
	Ceil    uint64
	Buffer  uint32
	Cbuffer uint32
	Quantum uint32
	Level   uint32
	Prio    uint32
}

func (q HtbClassAttrs) String() string {
	return fmt.Sprintf("{Rate: %d, Ceil: %d, Buffer: %d, Cbuffer: %d}", q.Rate, q.Ceil, q.Buffer, q.Cbuffer)
}

// HtbClass represents an Htb class
type HtbClass struct {
	ClassAttrs
	Rate    uint64
	Ceil    uint64
	Buffer  uint32
	Cbuffer uint32
	Quantum uint32
	Level   uint32
	Prio    uint32
}

func (q HtbClass) String() string {
	return fmt.Sprintf("{Rate: %d, Ceil: %d, Buffer: %d, Cbuffer: %d}", q.Rate, q.Ceil, q.Buffer, q.Cbuffer)
}

func (q *HtbClass) Attrs() *ClassAttrs {
	return &q.ClassAttrs
}

func (q *HtbClass) Type() string {
	return "htb"
}

// GenericClass classes represent types that are not currently understood
// by this netlink library.
type GenericClass struct {
	ClassAttrs
	ClassType string
}

func (class *GenericClass) Attrs() *ClassAttrs {
	return &class.ClassAttrs
}

func (class *GenericClass) Type() string {
	return class.ClassType
}
