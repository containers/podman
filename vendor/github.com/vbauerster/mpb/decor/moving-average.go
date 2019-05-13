package decor

import (
	"sort"

	"github.com/VividCortex/ewma"
)

// MovingAverage is the interface that computes a moving average over
// a time-series stream of numbers. The average may be over a window
// or exponentially decaying.
type MovingAverage interface {
	Add(float64)
	Value() float64
	Set(float64)
}

type medianWindow [3]float64

func (s *medianWindow) Len() int           { return len(s) }
func (s *medianWindow) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s *medianWindow) Less(i, j int) bool { return s[i] < s[j] }

func (s *medianWindow) Add(value float64) {
	s[0], s[1] = s[1], s[2]
	s[2] = value
}

func (s *medianWindow) Value() float64 {
	tmp := *s
	sort.Sort(&tmp)
	return tmp[1]
}

func (s *medianWindow) Set(value float64) {
	for i := 0; i < len(s); i++ {
		s[i] = value
	}
}

// NewMedian is fixed last 3 samples median MovingAverage.
func NewMedian() MovingAverage {
	return new(medianWindow)
}

type medianEwma struct {
	count  uint
	median MovingAverage
	MovingAverage
}

func (s *medianEwma) Add(v float64) {
	s.median.Add(v)
	if s.count >= 2 {
		s.MovingAverage.Add(s.median.Value())
	}
	s.count++
}

// NewMedianEwma is ewma based MovingAverage, which gets its values
// from median MovingAverage.
func NewMedianEwma(age ...float64) MovingAverage {
	return &medianEwma{
		MovingAverage: ewma.NewMovingAverage(age...),
		median:        NewMedian(),
	}
}
