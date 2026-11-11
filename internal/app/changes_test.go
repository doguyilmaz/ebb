package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/doguyilmaz/ebb/internal/fsevents"
)

func TestChangesCollectsFoldersUnderTheRoot(t *testing.T) {
	c := newChanges(func(p string) (string, bool) {
		if strings.HasPrefix(p, "/Users") {
			return "/System/Volumes/Data" + p, true
		}
		return "", false
	}, "/System/Volumes/Data/Users/me/.local/state/ebb")
	c.add([]fsevents.Event{
		{Path: "/Users/me/Downloads", ID: 10},
		{Path: "/Users/me/Downloads", ID: 11},
		{Path: "/Applications-elsewhere", ID: 12},
		{Path: "/Users/me/node_modules", Flags: fsevents.MustScanSubDirs, ID: 13},
		{Path: "/Users/me/.local/state/ebb", ID: 14},
		{Path: "", Flags: fsevents.HistoryDone, ID: 15},
	})
	if r := c.report(); r.Folders != 2 || r.Lost || !r.Ready {
		t.Fatalf("report %+v", r)
	}
	s, ok, _ := c.takeSettled()
	if !ok || fmt.Sprint(s.dirs, s.subtrees, s.last) != "[/System/Volumes/Data/Users/me/Downloads] [/System/Volumes/Data/Users/me/node_modules] 15" {
		t.Fatalf("took %v %v %d (ok %v)", s.dirs, s.subtrees, s.last, ok)
	}
	if r := c.report(); r.Folders != 0 {
		t.Fatal("taking empties the set")
	}
	c.add([]fsevents.Event{{Path: "/Users/me", Flags: fsevents.KernelDropped, ID: 16}})
	if r := c.report(); !r.Lost || r.LostWhy != "the kernel dropped events" {
		t.Fatalf("dropped events the history may lack mean the next scan is complete: %+v", r)
	}
}

// userDropped is what the live stream reports when it dropped events: the
// root path, with MustScanSubDirs.
func userDropped(id uint64) fsevents.Event {
	return fsevents.Event{Path: "/Users/me", Flags: fsevents.UserDropped | fsevents.MustScanSubDirs, ID: id}
}

func newTestChanges() *changes {
	return newChanges(func(p string) (string, bool) { return p, strings.HasPrefix(p, "/Users") }, "/Users/me/.local/state/ebb")
}

func TestUserDroppedIsRecoveredNotLost(t *testing.T) {
	c := newTestChanges()
	c.good = 5
	c.add([]fsevents.Event{{Path: "/Users/me/a", ID: 10}, {Flags: fsevents.HistoryDone}})
	c.add([]fsevents.Event{{Path: "/Users/me/b", ID: 20}, userDropped(25), {Path: "/Users/me/late", ID: 26}, userDropped(27)})
	r := c.report()
	if r.Lost || r.Ready {
		t.Fatalf("after a drop the set is neither lost nor complete: %+v", r)
	}
	if _, ok, wait := c.takeSettled(); ok || !wait {
		t.Fatalf("handed out while the gap is open: ok %v wait %v", ok, wait)
	}
	select {
	case <-c.restart:
	default:
		t.Fatal("a restart was not asked for")
	}
	select {
	case <-c.restart:
		t.Fatal("two drops in a row ask for one restart")
	default:
	}
	// The events between the drop and the restart do not move the point the replay starts from.
	if from, ok := c.restarted(); !ok || from != 20 {
		t.Fatalf("restart from %d (%v)", from, ok)
	}
	c.add([]fsevents.Event{{Path: "/Users/me/b", ID: 20}, {Path: "/Users/me/only-in-history", ID: 22}, {Flags: fsevents.HistoryDone}})
	s, ok, wait := c.takeSettled()
	if !ok || wait {
		t.Fatalf("after the replay: ok %v wait %v", ok, wait)
	}
	want := []string{"/Users/me/a", "/Users/me/b", "/Users/me/late", "/Users/me/only-in-history"}
	if !slices.Equal(s.dirs, want) {
		t.Fatalf("took %v, want %v", s.dirs, want)
	}
}

func TestEveryOtherLossStillMeansAFullScan(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags uint32
		why   string
	}{
		{"kernel", fsevents.KernelDropped, "the kernel dropped events"},
		{"wrap", fsevents.IDsWrapped, "the event IDs wrapped around"},
		{"root", fsevents.RootChanged, "the scanned folder was moved or replaced"},
		{"both", fsevents.UserDropped | fsevents.KernelDropped, "the kernel dropped events"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestChanges()
			c.add([]fsevents.Event{{Path: "/Users/me/a", ID: 10}, {Flags: fsevents.HistoryDone}, {Path: "/Users/me", Flags: tc.flags, ID: 11}})
			r := c.report()
			if !r.Lost || !r.Ready || r.LostWhy != tc.why {
				t.Fatalf("%+v", r)
			}
			if len(c.restart) != 0 {
				t.Fatal("asked for a restart")
			}
			if _, ok, wait := c.takeSettled(); ok || wait {
				t.Fatalf("lost sets are not handed out and not waited for: ok %v wait %v", ok, wait)
			}
		})
	}
}

func TestDropWhileHistoryReplaysRestartsFromTheProgressMade(t *testing.T) {
	// During the first replay after the scan ...
	c := newTestChanges()
	c.good = 5
	c.add([]fsevents.Event{{Path: "/Users/me/a", ID: 10}, userDropped(11)})
	if r := c.report(); r.Lost || r.Ready {
		t.Fatalf("%+v", r)
	}
	if from, ok := c.restarted(); !ok || from != 10 {
		t.Fatalf("restart from %d (%v)", from, ok)
	}
	// ... and during the replay that fills the gap after a drop.
	c = newTestChanges()
	c.good = 5
	c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}, userDropped(11)})
	if from, ok := c.restarted(); !ok || from != 10 {
		t.Fatalf("restart from %d (%v)", from, ok)
	}
	c.add([]fsevents.Event{{Path: "/Users/me/b", ID: 13}, userDropped(14)})
	if r := c.report(); r.Lost || r.Ready {
		t.Fatalf("a dropped replay is tried again: %+v", r)
	}
	if from, ok := c.restarted(); !ok || from != 13 {
		t.Fatalf("restart from %d (%v): the events the replay did deliver count", from, ok)
	}
	c.add([]fsevents.Event{{Path: "/Users/me/c", ID: 15}, {Flags: fsevents.HistoryDone}})
	if s, ok, wait := c.takeSettled(); !ok || wait || !slices.Equal(s.dirs, []string{"/Users/me/a", "/Users/me/b", "/Users/me/c"}) {
		t.Fatalf("took %v (ok %v wait %v)", s.dirs, ok, wait)
	}
}

func TestWaiterWakesWhenADroppedReplayIsRecovered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTestChanges()
		c.good = 5
		woke := make(chan bool, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			woke <- c.waitReady(ctx)
		}()
		// The waiter holds the channel of the replay since the scan when that replay is dropped.
		synctest.Wait()
		c.add([]fsevents.Event{{Path: "/Users/me/a", ID: 10}, userDropped(11)})
		if _, ok := c.restarted(); !ok {
			t.Fatal("not restarted")
		}
		c.add([]fsevents.Event{{Path: "/Users/me/b", ID: 12}, {Flags: fsevents.HistoryDone}})
		if !<-woke {
			t.Fatal("the waiter slept through the end of the recovery until its deadline")
		}
	})
}

func TestRepeatedDropsAreLost(t *testing.T) {
	c := newTestChanges()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	c.good = 5
	c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}})
	id := uint64(10)
	drop := func() {
		c.add([]fsevents.Event{{Path: "/Users/me/a", ID: id}, userDropped(id + 1)})
		if _, ok := c.restarted(); !ok {
			t.Fatal("not restarted")
		}
		c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}})
		id += 10
		now = now.Add(time.Minute)
	}
	for range maxRecoveries {
		drop()
	}
	if r := c.report(); r.Lost || !r.Ready {
		t.Fatalf("%d recoveries are within the limit: %+v", maxRecoveries, r)
	}
	c.add([]fsevents.Event{userDropped(id)})
	if r := c.report(); !r.Lost || !strings.Contains(r.LostWhy, "more than") {
		t.Fatalf("one more drop within the window: %+v", r)
	}

	// Drops spread over hours are not repeats.
	c = newTestChanges()
	c.now = func() time.Time { return now }
	c.good = 5
	c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}})
	for range 3 * maxRecoveries {
		c.add([]fsevents.Event{{Path: "/Users/me/a", ID: id}, userDropped(id + 1)})
		if _, ok := c.restarted(); !ok {
			t.Fatalf("lost: %+v", c.report())
		}
		c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}})
		id += 10
		now = now.Add(recoveryWindow)
	}
	if r := c.report(); r.Lost {
		t.Fatalf("%+v", r)
	}
}

func TestDropWithoutAPointToReplayFromIsLost(t *testing.T) {
	c := newTestChanges()
	c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}, userDropped(11)})
	if r := c.report(); !r.Lost || !r.Ready {
		t.Fatalf("starting a stream at 0 would mean 'from now': %+v", r)
	}
}

func TestDropNamingAnEarlierPointReplaysFromJustBeforeIt(t *testing.T) {
	c := newTestChanges()
	c.good = 5
	c.add([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 30}, userDropped(20)})
	if from, ok := c.restarted(); !ok || from != 19 {
		t.Fatalf("restart from %d (%v)", from, ok)
	}
}

// fakeStreams stands in for FSEvents: every stream the watcher starts plays
// the next script, then serves flush requests until it is stopped.
type fakeStreams struct {
	mu      sync.Mutex
	since   []uint64
	scripts []func(deliver func([]fsevents.Event)) error
	flushed int
	stopped int
}

func (f *fakeStreams) watch(ctx context.Context, _ string, since uint64, _ time.Duration, fn func([]fsevents.Event), flush <-chan chan struct{}) error {
	f.mu.Lock()
	i := len(f.since)
	f.since = append(f.since, since)
	var play func(func([]fsevents.Event)) error
	if i < len(f.scripts) {
		play = f.scripts[i]
	}
	f.mu.Unlock()
	if play != nil {
		if err := play(fn); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			f.mu.Lock()
			f.stopped++
			f.mu.Unlock()
			return nil
		case done := <-flush:
			f.mu.Lock()
			f.flushed++
			f.mu.Unlock()
			close(done)
		}
	}
}

func (f *fakeStreams) sinces() []uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.since)
}

func (f *fakeStreams) count(p *int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *p
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// startFake follows a set with scripted streams that began after event 5.
func startFake(t *testing.T, f *fakeStreams) *watcher {
	t.Helper()
	set := newTestChanges()
	set.good = 5
	set.calmFor, set.calmMax = 20*time.Millisecond, 200*time.Millisecond
	w := startWatcher(context.Background(), "/Users/me", 5, set, f.watch)
	t.Cleanup(func() {
		w.stop()
		<-w.ended
	})
	return w
}

func TestWatcherRestartsFromTheLastGoodEventAfterADrop(t *testing.T) {
	release := make(chan struct{})
	f := &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}, {Path: "/Users/me/b", ID: 20}, userDropped(25)})
			return nil
		},
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Path: "/Users/me/b", ID: 20}, {Path: "/Users/me/only-in-history", ID: 22}})
			<-release
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/live", ID: 30}})
			return nil
		},
	}}
	w := startFake(t, f)
	eventually(t, "the second stream", func() bool { return len(f.sinces()) == 2 })
	if got := f.sinces(); !slices.Equal(got, []uint64{5, 20}) {
		t.Fatalf("streams started after %v: the replay starts at the last event before the drop", got)
	}
	eventually(t, "the first stream to be stopped", func() bool { return f.count(&f.stopped) == 1 })
	eventually(t, "the replayed folder", func() bool { return w.set.report().Folders == 3 })
	if r := w.set.report(); r.Lost || r.Ready {
		t.Fatalf("while the replay runs the set is not complete: %+v", r)
	}
	if _, ok, wait := w.set.takeSettled(); ok || !wait {
		t.Fatalf("handed out during the replay: ok %v wait %v", ok, wait)
	}
	waited := make(chan bool, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		waited <- w.set.waitReady(ctx)
	}()
	select {
	case <-waited:
		t.Fatal("waitReady returned during the replay")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if !<-waited {
		t.Fatal("waitReady did not see the replay end")
	}
	s, ok, wait := w.set.takeSettled()
	want := []string{"/Users/me/a", "/Users/me/b", "/Users/me/live", "/Users/me/only-in-history"}
	if !ok || wait || !slices.Equal(s.dirs, want) {
		t.Fatalf("took %v (ok %v wait %v), want %v", s.dirs, ok, wait, want)
	}
}

func TestWatcherKeepsFlushingAfterARestart(t *testing.T) {
	f := &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}, userDropped(11)})
			return nil
		},
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}})
			return nil
		},
	}}
	w := startFake(t, f)
	eventually(t, "the set to be complete again", func() bool { return len(f.sinces()) == 2 && w.set.report().Ready })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.sync(ctx)
	if f.count(&f.flushed) != 1 {
		t.Fatal("the new stream did not flush when asked")
	}
}

func TestWatcherLosesTheSetWhenTheDropCannotBeRecovered(t *testing.T) {
	t.Run("kernel drop does not restart", func(t *testing.T) {
		f := &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
			func(deliver func([]fsevents.Event)) error {
				deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}, {Path: "/Users/me", Flags: fsevents.KernelDropped, ID: 11}})
				return nil
			},
		}}
		w := startFake(t, f)
		eventually(t, "lost", func() bool { return w.set.report().Lost })
		time.Sleep(100 * time.Millisecond)
		if got := f.sinces(); len(got) != 1 {
			t.Fatalf("streams %v", got)
		}
	})
	t.Run("a replay that is dropped over and over", func(t *testing.T) {
		var scripts []func(func([]fsevents.Event)) error
		for i := range maxRecoveries + 2 {
			id := uint64(10 + 10*i)
			scripts = append(scripts, func(deliver func([]fsevents.Event)) error {
				deliver([]fsevents.Event{{Path: "/Users/me/a", ID: id}, userDropped(id + 1)})
				return nil
			})
		}
		f := &fakeStreams{scripts: scripts}
		w := startFake(t, f)
		eventually(t, "lost", func() bool { return w.set.report().Lost })
		if r := w.set.report(); !r.Ready || !strings.Contains(r.LostWhy, "more than") {
			t.Fatalf("%+v", r)
		}
		if got := f.sinces(); len(got) != maxRecoveries+1 || got[1] != 10 || got[2] != 20 {
			t.Fatalf("each try starts where the last one got to: %v", got)
		}
	})
	t.Run("a stream that fails to start", func(t *testing.T) {
		f := &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
			func(deliver func([]fsevents.Event)) error {
				deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}, userDropped(11)})
				return nil
			},
			func(func([]fsevents.Event)) error { return errors.New("no stream") },
		}}
		w := startFake(t, f)
		eventually(t, "lost", func() bool { return w.set.report().Lost })
		if r := w.set.report(); !r.Ready || !strings.Contains(r.LostWhy, "no stream") {
			t.Fatalf("%+v", r)
		}
		<-w.ended
	})
}

func TestWatcherRecoversFromADroppedReplay(t *testing.T) {
	f := &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}, userDropped(11)})
			return nil
		},
		func(deliver func([]fsevents.Event)) error {
			// A replay while the burst is still on: a few events, then a drop.
			deliver([]fsevents.Event{{Path: "/Users/me/a", ID: 10}, {Path: "/Users/me/b", ID: 12}, userDropped(13)})
			return nil
		},
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Path: "/Users/me/c", ID: 14}, {Flags: fsevents.HistoryDone}})
			return nil
		},
	}}
	w := startFake(t, f)
	eventually(t, "the set to be complete again", func() bool { return len(f.sinces()) == 3 && w.set.report().Ready })
	if got := f.sinces(); !slices.Equal(got, []uint64{5, 10, 12}) {
		t.Fatalf("streams started after %v", got)
	}
	s, ok, _ := w.set.takeSettled()
	if !ok || !slices.Equal(s.dirs, []string{"/Users/me/a", "/Users/me/b", "/Users/me/c"}) {
		t.Fatalf("took %v (ok %v)", s.dirs, ok)
	}
}

func TestWatcherWaitsForDropsToStopBeforeRestarting(t *testing.T) {
	const calm = 150 * time.Millisecond
	var mu sync.Mutex
	var firstDrop, lastDrop time.Time
	f := &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}})
			// A burst: a drop every 15 ms for 100 ms, so the calm period starts late.
			for i := range 7 {
				mu.Lock()
				if firstDrop.IsZero() {
					firstDrop = time.Now()
				}
				lastDrop = time.Now()
				mu.Unlock()
				deliver([]fsevents.Event{userDropped(uint64(20 + i))})
				time.Sleep(15 * time.Millisecond)
			}
			return nil
		},
	}}
	set := newTestChanges()
	set.good = 5
	set.calmFor, set.calmMax = calm, 5*time.Second
	w := startWatcher(context.Background(), "/Users/me", 5, set, f.watch)
	t.Cleanup(func() {
		w.stop()
		<-w.ended
	})
	eventually(t, "the first drop", func() bool { mu.Lock(); defer mu.Unlock(); return !firstDrop.IsZero() })
	// The old stream still answers flushes while the restart waits.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.sync(ctx)
	if n := len(f.sinces()); n != 1 || f.count(&f.flushed) != 1 {
		t.Fatalf("%d streams, %d flushes: the restart came before the wait was over, or the old stream did not flush", n, f.count(&f.flushed))
	}
	eventually(t, "the second stream", func() bool { return len(f.sinces()) == 2 })
	mu.Lock()
	quiet := time.Since(lastDrop)
	mu.Unlock()
	if quiet < calm {
		t.Fatalf("restarted %v after the last drop, before the %v wait was over", quiet, calm)
	}
	if got := f.sinces(); got[1] != 10 {
		t.Fatalf("streams started after %v", got)
	}
}

func TestWatcherRestartsAfterTheLongestWaitWhenDropsNeverStop(t *testing.T) {
	stop := make(chan struct{})
	var f *fakeStreams
	f = &fakeStreams{scripts: []func(func([]fsevents.Event)) error{
		func(deliver func([]fsevents.Event)) error {
			deliver([]fsevents.Event{{Flags: fsevents.HistoryDone}, {Path: "/Users/me/a", ID: 10}})
			go func() {
				for i := uint64(0); len(f.sinces()) < 2; i++ {
					select {
					case <-stop:
						return
					case <-time.After(10 * time.Millisecond):
						deliver([]fsevents.Event{userDropped(20 + i)})
					}
				}
			}()
			return nil
		},
	}}
	t.Cleanup(func() { close(stop) })
	set := newTestChanges()
	set.good = 5
	set.calmFor, set.calmMax = 100*time.Millisecond, 300*time.Millisecond
	w := startWatcher(context.Background(), "/Users/me", 5, set, f.watch)
	t.Cleanup(func() {
		w.stop()
		<-w.ended
	})
	eventually(t, "the second stream", func() bool { return len(f.sinces()) == 2 })
}
