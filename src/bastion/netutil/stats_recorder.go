package netutil

import (
	"time"
    "log"
)

type StatsRecorder interface {
	Timer(stat string, amount int64)
	DurationTimer(stat string, begin time.Time, end time.Time)
	Gauge(stat string, amount int64)
	Counter(stat string, amount int64)
	Increment(stat string)
}

type DebugStatsRecorder struct{}

func (s *DebugStatsRecorder) log(stat string, amount int64) {
	log.Printf("stats %s %v", stat, amount)
}

func (s *DebugStatsRecorder) Timer(stat string, amount int64) {
	s.log(stat, amount)
}

func (s *DebugStatsRecorder) DurationTimer(stat string, begin time.Time, end time.Time) {
	amount := int64(end.Sub(begin) / time.Millisecond)
	s.log(stat, amount)
}

func (s *DebugStatsRecorder) Gauge(stat string, amount int64) {
	s.log(stat, amount)
}

func (s *DebugStatsRecorder) Counter(stat string, amount int64) {
	s.log(stat, amount)
}

func (s *DebugStatsRecorder) Increment(stat string) {
	s.log(stat, 1)
}
