// Package daemon is ebb's background service: it serves the window and the
// API, follows what changes on disk, keeps the last scan fresh while nobody
// uses the Mac and warns when free space runs low.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/app"
	"github.com/doguyilmaz/ebb/internal/config"
	"github.com/doguyilmaz/ebb/internal/server"
	"github.com/doguyilmaz/ebb/internal/svc"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/units"
)

type Options struct {
	Version string
	Port    int           // 0: from the config
	Every   time.Duration // between checks; 0: five minutes
	Out     io.Writer     // log lines; launchd sends them to daemon.log
}

const maxLog = 5 << 20

type daemon struct {
	a     *app.App
	s     *server.Server
	opt   Options
	dir   string
	st    api.Daemon
	alert Alert
}

// ErrRunning: one service per state folder, or two would scan and serve at once.
var ErrRunning = errors.New("the background service is already running")

// Run serves until ctx is done. While another ebb holds the window (an
// `ebb serve` in a terminal) it waits for that to close instead of exiting,
// which launchd would answer by starting it again at once.
func Run(ctx context.Context, a *app.App, opt Options) error {
	env := a.Env
	d := &daemon{a: a, opt: opt, dir: sys.StateDir(env), st: api.Daemon{PID: os.Getpid(), Started: env.Now()}}
	if cur := svc.Local(env).Read(); cur.Running && cur.PID != os.Getpid() {
		return ErrRunning
	}
	if d.opt.Every == 0 {
		d.opt.Every = 5 * time.Minute
	}
	if d.opt.Out == nil {
		d.opt.Out = io.Discard
	}
	svc.TrimLog(d.dir, maxLog)
	defer func() {
		d.st.PID, d.st.Waiting = 0, false
		d.save()
	}()
	if !d.waitForWindow(ctx) {
		return nil
	}
	token, err := server.LoadOrCreateToken(env)
	if err != nil {
		return err
	}
	port := opt.Port
	if port == 0 {
		port = a.Config.Server.Port
	}
	d.s = server.New(a, opt.Version, token)
	served := make(chan error, 1)
	go func() {
		served <- server.Serve(ctx, d.s, fmt.Sprintf("127.0.0.1:%d", port), func(string) {
			_ = server.WriteStatus(env, server.Status{PID: os.Getpid(), Port: port, Version: opt.Version, Started: d.st.Started, Daemon: true})
			d.logf("serving on 127.0.0.1:%d", port)
		})
	}()
	defer server.RemoveStatus(env)
	go a.Watch(ctx)
	d.save()
	// The first check waits a round, so the change watcher has caught up.
	tick := time.NewTicker(d.opt.Every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-served:
			if ctx.Err() != nil {
				return nil
			}
			d.st.Error = "could not serve the window: " + err.Error()
			d.save()
			d.logf("%s", d.st.Error)
			// launchd starts it again right away; a pause keeps a busy port from flooding the log.
			select {
			case <-ctx.Done():
			case <-time.After(time.Minute):
			}
			return err
		case <-tick.C:
			d.check(ctx)
		}
	}
}

func (d *daemon) waitForWindow(ctx context.Context) bool {
	for {
		if _, live := server.Live(d.a.Env); !live {
			d.st.Waiting = false
			return true
		}
		if !d.st.Waiting {
			d.st.Waiting = true
			d.save()
			d.logf("another ebb serves the window; waiting for it to close")
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Minute):
		}
	}
}

func (d *daemon) check(ctx context.Context) {
	env := d.a.Env
	cfg, err := config.Load(env)
	if err != nil {
		cfg = d.a.Config
	}
	now := env.Now()
	d.st.LastCheck = now
	defer d.save()
	if f, ok := free(env.Home()); ok && cfg.Daemon.LowSpaceGB > 0 && d.alert.Check(f, uint64(cfg.Daemon.LowSpaceGB)*1e9) {
		d.st.LastAlert = now
		if err := notify(ctx, env, fmt.Sprintf("Only %s left on this Mac. Open ebb to see what can go.", units.Bytes(f))); err != nil {
			d.logf("low space notification: %v", err)
		}
	}
	if !cfg.Daemon.Scans {
		return
	}
	last, err := d.a.Last()
	if err != nil {
		return
	}
	c := Conditions{Now: now, LastScan: last.Meta.Time, LastFull: d.lastFull(last), OnAC: onAC(ctx, env), Idle: idle(ctx, env), Lost: true}
	if ch := d.a.Changes(); ch != nil {
		c.Changed, c.Pending, c.Lost = ch.Folders, !ch.Ready, ch.Lost
	}
	work := NextWork(c)
	if work == Nothing {
		return
	}
	err = d.s.StartScan(app.ScanOptions{Root: last.Meta.Root, Gentle: true, Full: work == Full})
	if errors.Is(err, app.ErrScanRunning) || errors.Is(err, app.ErrCleaning) {
		return
	}
	if err != nil {
		d.logf("%s scan: %v", work, err)
		return
	}
	d.st.LastWork, d.st.LastWorkAt = work.String(), now
	d.logf("%s scan started", work)
}

// lastFull is when root was last read in full; never means now.
func (d *daemon) lastFull(last *app.ScanResult) time.Time {
	if !last.Meta.Incremental {
		return last.Meta.Time
	}
	metas, _ := d.a.History.List()
	for _, m := range metas {
		if !m.Incremental && m.Root == last.Meta.Root {
			return m.Time
		}
	}
	return time.Time{}
}

func (d *daemon) save() { _ = svc.WriteStatus(d.dir, d.st) }

func (d *daemon) logf(format string, args ...any) {
	fmt.Fprintf(d.opt.Out, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
