package daemon

import "time"

// Work is what the background service does next.
type Work int

const (
	Nothing Work = iota
	Update       // list again only the folders that changed
	Full         // read every folder
)

func (w Work) String() string { return [...]string{"nothing", "update", "full"}[w] }

// Conditions are what the decision depends on.
type Conditions struct {
	Now      time.Time
	OnAC     bool
	Idle     time.Duration
	Changed  int  // folders changed since the last scan
	Known    bool // the watcher knows what changed (history replayed, nothing lost)
	LastScan time.Time
	LastFull time.Time
}

const (
	minIdle  = 10 * time.Minute
	minGap   = 30 * time.Minute
	fullGap  = 7 * 24 * time.Hour
	headroom = 2e9
)

// NextWork keeps results fresh without getting in anyone's way: only on power
// and while nobody uses the Mac, at most every half hour, an update when
// something changed and a full scan once a week (or when changes are unknown)
// to correct any drift. The first scan is always the person's own.
func NextWork(c Conditions) Work {
	switch {
	case c.LastScan.IsZero(), !c.OnAC, c.Idle < minIdle, c.Now.Sub(c.LastScan) < minGap:
		return Nothing
	case !c.Known, c.Now.Sub(c.LastFull) >= fullGap:
		return Full
	case c.Changed > 0:
		return Update
	}
	return Nothing
}

// Alert says when to warn about low space: once when free space drops below
// the threshold, and again only after it climbed back well above it.
type Alert struct{ fired bool }

func (a *Alert) Check(free, threshold uint64) bool {
	switch {
	case !a.fired && free < threshold:
		a.fired = true
		return true
	case a.fired && free > threshold+headroom:
		a.fired = false
	}
	return false
}
