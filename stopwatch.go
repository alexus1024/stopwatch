package stopwatch

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

func defaultFormatter(duration time.Duration) string {
	return duration.String()
}

// Stopwatch is a non high-resolution timer for recording elapsed time deltas
// to give you some insight into how long things take for your app
type Stopwatch struct {
	start, stop    time.Time     // no need for lap, see mark
	mark           time.Duration // mark is the duration from the start that the most recent lap was started
	laps           []Lap         //
	formatter      func(time.Duration) string
	formattingMode FormattingMode
	sync.RWMutex
}

type FormattingMode string

const (
	// FormattingModeJsonArray formats Stopwatch to the form of an array of laps [{},{},...]
	FormattingModeJsonArray FormattingMode = "JSON_ARRAY"
	// FormattingModeJsonSimpleObject formats Stopwatch to the form object with property-per-lap {"Lap1":"10ms", "Lap2":"20ms"}
	// It's compatitable with ELK. Does not support additional lap data
	FormattingModeJsonSimpleObject FormattingMode = "JSON_OBJECT"
	// FormattingModeJsonMsObject formats Stopwatch to the form object with property-per-lap, where values are numbers {"Lap1":10.1, "Lap2":20.2}
	// It's compatitable with ELK. Does not support additional lap data
	FormattingModeJsonMsObject FormattingMode = "JSON_OBJECT_MS"

	defaultFormattingMode FormattingMode = FormattingModeJsonArray
)

// New creates a new stopwatch with starting time offset by
// a user defined value. Negative offsets result in a countdown
// prior to the start of the stopwatch.
func New(offset time.Duration, active bool) *Stopwatch {
	var sw Stopwatch
	sw.Reset(offset, active)
	sw.SetFormatter(defaultFormatter)
	sw.SetFormattingMode(defaultFormattingMode)
	return &sw
}

// SetFormatter takes a function that converts time.Duration into a string
func (s *Stopwatch) SetFormatter(formatter func(time.Duration) string) {
	s.Lock()
	s.formatter = formatter
	s.Unlock()
}

// MarshalJSON converts into a slice of bytes
func (s *Stopwatch) MarshalJSON() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Stopwatch) String() string {

	s.RLock()
	defer s.RUnlock()

	switch defaultedFormattingMode(s.formattingMode) {
	case FormattingModeJsonSimpleObject:
		return s.formatAsObject(func(lap Lap) string {
			return fmt.Sprintf(`"%s":"%s"`, lap.state, lap.formatter(lap.duration))
		})

	case FormattingModeJsonMsObject:
		return s.formatAsObject(func(lap Lap) string {
			return fmt.Sprintf(`"%s":%.3f`, lap.state, float64(lap.duration.Microseconds())/1000.0) // ms 1234.567
		})

	case FormattingModeJsonArray:
		fallthrough
	default:
		results := make([]string, len(s.laps))
		for i, v := range s.laps {
			results[i] = v.String()
		}
		return fmt.Sprintf("[%s]", strings.Join(results, ", "))
	}

}

func (s *Stopwatch) formatAsObject(lapValueFormatter func(Lap) string) string {
	results := make([]string, len(s.laps))
	for i, lap := range s.laps {
		results[i] = lapValueFormatter(lap)
	}
	return fmt.Sprintf("{%s}", strings.Join(results, ", "))
}

// Reset allows the re-use of a Stopwatch instead of creating
// a new one.
func (s *Stopwatch) Reset(offset time.Duration, active bool) {
	now := time.Now()
	s.Lock()
	defer s.Unlock()
	s.start = now.Add(-offset)
	if active {
		s.stop = time.Time{}
	} else {
		s.stop = now
	}
	s.mark = 0
	s.laps = nil
}

// Active returns true if the stopwatch is active (counting up)
func (s *Stopwatch) active() bool {
	return s.stop.IsZero()
}

// Stop makes the stopwatch stop counting up
func (s *Stopwatch) Stop() {
	s.Lock()
	defer s.Unlock()
	if s.active() {
		s.stop = time.Now()
	}
}

// Start intiates, or resumes the counting up process
func (s *Stopwatch) Start() {
	s.Lock()
	defer s.Unlock()
	if !s.active() {
		diff := time.Since(s.stop)
		s.start = s.start.Add(diff)
		s.stop = time.Time{}
	}
}

// ElapsedTime is the time the stopwatch has been active
func (s *Stopwatch) ElapsedTime() time.Duration {
	if s.active() {
		return time.Since(s.start)
	}
	return s.stop.Sub(s.start)
}

// ElapsedTimeFrom is the time the stopwatch has been active till 'now'
func (s *Stopwatch) ElapsedTimeFrom(now time.Time) time.Duration {
	if s.active() {
		return now.Sub(s.start)
	}
	return s.stop.Sub(s.start)
}

// LapTime is the time since the start of the lap
func (s *Stopwatch) LapTime() time.Duration {
	s.RLock()
	defer s.RUnlock()
	return s.ElapsedTime() - s.mark
}

// Lap starts a new lap, and returns the length of
// the previous one.
func (s *Stopwatch) Lap(state string) Lap {
	return s.LapWithData(state, nil)
}

// LapWithData starts a new lap, and returns the length of
// the previous one allowing the user to pass in additional
// metadata to be recorded.
func (s *Stopwatch) LapWithData(state string, data map[string]interface{}) Lap {
	return s.LapWithDataAndTime(time.Now(), state, data)
}

// LapWithDataAndTime starts a new lap from 'now' timestamp, and returns the length of
// the previous one allowing the user to pass in additional
// metadata to be recorded.
func (s *Stopwatch) LapWithDataAndTime(now time.Time, state string, data map[string]interface{}) Lap {
	s.Lock()
	defer s.Unlock()
	elapsed := s.ElapsedTimeFrom(now)
	lap := Lap{
		formatter: s.formatter,
		state:     state,
		duration:  elapsed - s.mark,
		data:      data,
	}
	s.mark = elapsed
	s.laps = append(s.laps, lap)
	return lap
}

// Laps returns a slice of completed lap times
func (s *Stopwatch) Laps() []Lap {
	s.RLock()
	defer s.RUnlock()
	laps := make([]Lap, len(s.laps))
	copy(laps, s.laps)
	return laps
}

// Laps returns a slice of completed lap times
func (s *Stopwatch) SetFormattingMode(newMode FormattingMode) {
	s.Lock()
	defer s.Unlock()
	s.formattingMode = newMode
}

func defaultedFormattingMode(src FormattingMode) FormattingMode {

	if src == "" {
		return defaultFormattingMode
	}

	return src
}
