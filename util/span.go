// span is a very simple implementation of ideas presented in Google's Dapper
// paper (http://research.google.com/pubs/pub36356.html). The basic idea
// is that every request has a Span object associated with it which allows
// for registering subspans (used for timing subroutines), attributes (arbitrary
// metadata) and counts (incrementers ala StatsD). At the end of a request,
// this span is turned into JSON and logged to logger as well as the timings
// and counters sent through the supplied StatsRecorder.
package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/satori/go.uuid"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// A UUIDGenerator is a func that returns a unique id as a string
type UUIDGenerator func() string

// The DefaultUUIDGenerator uses "github.com/satori/go.uuid"/V1 to
// generate RFC 4122 compatible uuids
func DefaultUUIDGenerator() string {
	return uuid.NewV1().String()
}

// A SubSpan represents an unique subroutine and its start/end times. These
// are coallated at the end of a Span and turned into Durations.
type SubSpan struct {
	Name     string
	Started  time.Time
	Finished time.Time
}

func (s *SubSpan) Finish(started time.Time) time.Time {
	s.Started = started
	finished := time.Now()
	s.Finished = finished
	return finished
}

// The Duration of the SubSpan as an int64 representing nanoseconds
func (s *SubSpan) Duration() int64 {
	return int64(s.Finished.Sub(s.Started))
}

// The Duration of the subspan as a millisecond prescision float64
func (s *SubSpan) MillisecondDuration() float64 {
	return float64(s.Finished.Sub(s.Started)) / float64(time.Millisecond)
}

// Span is the main object passed through a RequestHandler that stores
// all the metadata about a request. It should be initialized with a UUID
// through NewSpan().
type Span struct {
	sync.Mutex
	Stats    StatsRecorder
	Id       string
	ParentId string
	SubSpans map[string]*SubSpan
	Counters map[string]int64
	Attrs    map[string]string
}

// Initialize a Span for a unique request with a UUID. This also initializes
// all of the substructures/maps for storing the metadata.
func NewSpan(id string) (s *Span) {
	s = new(Span)
	s.Id = id
	s.SubSpans = make(map[string]*SubSpan)
	s.Counters = make(map[string]int64)
	s.Attrs = make(map[string]string)
	s.Stats = new(DebugStatsRecorder)
	return s
}

// Start a subspan with name. names need to be unique per-Span as these
// are stored in a map of name->SubSpan. If you have multiple recouring calls
// to a subroutine in a request, consider naming them with `method-newuuid`
func (s *Span) Start(name string) {
	s.Lock()
	defer s.Unlock()
	sub := s.SubSpans[name]
	started := time.Now()
	if sub != nil {
		sub.Started = started
	} else {
		s.SubSpans[name] = &SubSpan{Name: name, Started: started}
	}
}

// Finish marks the subspan at name as finished, returning the duration (useful for debug logging).
// This does not have to be called in the same goroutine or location as the .Start() for the SubSpan,
// in fact, you can call Finish on an unstarted SubSpan without error (the duration will be 0).
func (s *Span) Finish(name string) (duration int64) {
	s.Lock()
	defer s.Unlock()
	sub := s.SubSpans[name]
	finished := time.Now()
	if sub != nil {
		sub.Finished = finished
	} else {
		sub = &SubSpan{Name: name, Started: finished, Finished: finished}
		s.SubSpans[name] = sub
	}
	return sub.Duration()
}

// SubSpanWithDuration creates a new subspan with the duration expressed as a float64 of milliseconds.
func (s *Span) SubSpanWithDuration(name string, msduration float64) {
	s.Lock()
	defer s.Unlock()
	sub := s.SubSpans[name]
	started := time.Now()
	dur := time.Duration(msduration * 1000 * 1000)
	finished := started.Add(dur)
	if sub != nil {
		sub.Started = started
		sub.Finished = finished
	} else {
		s.SubSpans[name] = &SubSpan{Name: name, Started: started, Finished: finished}
	}
}

// Add increments the counter at name by val. names for counters also are unique per-Span.
func (s *Span) Add(name string, val int64) int64 {
	s.Lock()
	defer s.Unlock()
	if counter, ok := s.Counters[name]; ok == true {
		s.Counters[name] = counter + val
	} else {
		s.Counters[name] = val
	}
	return s.Counters[name]
}

// Increment increments the counter at name by 1.
func (s *Span) Increment(name string) int64 {
	return s.Add(name, 1)
}

// Attr stores arbitrary metadata for the Span as a key/value map.
func (s *Span) Attr(k, v string) {
	s.Lock()
	defer s.Unlock()
	s.Attrs[k] = v
}

// SubSpan returns the SubSpan at name. If the SubSpan does not exist,
// it initializes a new one with name.
func (s *Span) SubSpan(name string) (sub *SubSpan) {
	s.Lock()
	defer s.Unlock()
	sub, ok := s.SubSpans[name]
	if ok != true {
		sub = &SubSpan{Name: name}
		s.SubSpans[name] = sub
	}
	return sub
}

// Duration returns the nanosecond duration of the SubSpan at name.
func (s *Span) Duration(name string) int64 {
	return s.SubSpan(name).Duration()
}

// MillisecondDuration() returns a float64 of the duration with millisecond prescision.
func (s *Span) MillisecondDuration(name string) float64 {
	return s.SubSpan(name).MillisecondDuration()
}

type SpanJSON map[string]interface{}

// MergeJSON imports values from a wellformed json that looks like
// {"subspans": {}, "attrs": {}, "counters": {}}
func (s *Span) MergeJSON(span string) (err error) {
	var sj SpanJSON
	err = json.Unmarshal([]byte(span), &sj)
	if err != nil {
		return err
	}
	for k, v := range sj["subspans"].(map[string]interface{}) {
		s.SubSpanWithDuration(k, v.(float64))
	}
	for k, v := range sj["attrs"].(map[string]interface{}) {
		s.Attr(k, v.(string))
	}
	for k, v := range sj["counters"].(map[string]interface{}) {
		s.Add(k, int64(v.(float64)))
	}
	return nil
}

// Record flushes the counters and SubSpan durations to the provider StatsRecorder.
// By default this goes to /dev/null, but using the StatsdStatsdRecorder this can
// be flushed to Statsd (or any other service that conforms to the StatsRecorder
// interface)
func (s *Span) Record() {
	if s.Stats != nil {
		for k, v := range s.Counters {
			s.Stats.Counter(k, v)
		}
		for k, v := range s.SubSpans {
			s.Stats.Timer(k, int64(v.MillisecondDuration()))
		}
	}
}

// JSON marshalls the Span into a JSON formatted string with all the subspans turned
// into their millisecond durations. This is the default for what is logged by the tcpez server.
func (s *Span) JSON() string {
	j := make(map[string]string)

	j["id"] = s.Id
	j["parentid"] = s.ParentId
	for k, v := range s.Attrs {
		j[k] = v
	}
	for k, v := range s.Counters {
		j[k] = fmt.Sprintf("%d", v)
	}
	for k, v := range s.SubSpans {
		j[k] = fmt.Sprintf("%f", v.MillisecondDuration())
	}
	b, _ := json.Marshal(j)
	return string(b)
}

// String turns the Span into a k=v formatted string with the subspans turned into
// their millisecond durations.
func (s *Span) String() string {
	b := bytes.NewBufferString("")

	for k, v := range s.Attrs {
		fmt.Fprintf(b, "%s=%s ", k, v)
	}
	for k, v := range s.Counters {
		fmt.Fprintf(b, "%s=%d ", k, v)
	}
	for k, v := range s.SubSpans {
		fmt.Fprintf(b, "%s=%fms ", k, v.MillisecondDuration())
	}
	return b.String()
}

// CollectMemStats populates the span with a number of memory statistics from
// runtime.MemStats. Call this before logging your span for info about your
// applications memory usage
func (s *Span) CollectMemStats() {
	s.Start("mem.duration")
	defer s.Finish("mem.duration")
	m := runtime.MemStats{}
	runtime.ReadMemStats(&m)
	r := new(syscall.Rusage)
	syscall.Getrusage(syscall.RUSAGE_SELF, r)
	s.Add("mem.alloc", int64(m.Alloc/1024))
	s.Add("mem.total_alloc", int64(m.TotalAlloc/1024))
	s.Add("mem.sys", int64(m.Sys/1024))
	s.Add("mem.heap_sys", int64(m.HeapSys/1024))
	s.Add("mem.heap_inuse", int64(m.HeapInuse/1024))
	s.Add("mem.heap_idle", int64(m.HeapIdle/1024))
	s.Add("mem.heap_released", int64(m.HeapReleased/1024))
	s.Add("mem.heap_objects", int64(m.HeapObjects/1024))
	s.Add("mem.num_gc", int64(m.NumGC/1024))
	s.Add("mem.max_rss", r.Maxrss)
}
