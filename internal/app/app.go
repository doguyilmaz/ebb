// Package app is the facade the CLI, the HTTP server and the MCP server
// share: one place that knows how to scan, detect, persist and answer.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/doguyilmaz/ebb"
	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/config"
	"github.com/doguyilmaz/ebb/internal/dupes"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/sources"
	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/volume"
)

const dataVolume = "/System/Volumes/Data"

var (
	ErrNeverScanned = errors.New("no scan yet — run: ebb scan")
	ErrScanRunning  = errors.New("a scan is already running")
	ErrScanStopped  = errors.New("the scan was stopped")
	ErrCleaning     = errors.New("a cleanup is running: try again when it is done")
)

type App struct {
	Env     sys.Env
	Config  config.Config
	Rules   *rules.Set
	History *history.Store
	Links   volume.Firmlinks

	mu       sync.Mutex
	running  bool
	cleaning bool
	packages struct {
		scan   string
		result sources.Packages
	}
	dupes struct {
		running, loaded bool
		stopped         bool
		stop            context.CancelFunc
		progress        *dupes.Progress
		last            *dupes.Result
		err             string
	}
	stop     context.CancelFunc
	watch    *watcher
	watchCtx context.Context
	scanRoot string
	scanErr  string
	progress *api.Progress
	cur      *ScanResult
	plans    map[string]*planEntry
}

func New(env sys.Env) (*App, error) {
	cfg, err := config.Load(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", config.Path(env), err)
	}
	var large rules.Size
	if err := large.UnmarshalText([]byte(cfg.Scan.LargeFile)); err != nil || large < 1<<20 {
		return nil, fmt.Errorf("%s: [scan] large_file = %q is not a size like \"1GiB\"", config.Path(env), cfg.Scan.LargeFile)
	}
	set, err := rules.Load(env, ebb.Rules)
	if err != nil {
		return nil, err
	}
	a := &App{Env: env, Config: cfg, Rules: set, History: history.Open(sys.StateDir(env))}
	if runtime.GOOS == "darwin" {
		a.Links = volume.LoadFirmlinks("/usr/share/firmlinks", dataVolume)
	}
	return a, nil
}

// DefaultRoot is the whole data volume on macOS, the configured root when
// set, and the sandbox home in a sandboxed session.
func (a *App) DefaultRoot() string {
	switch {
	case a.Config.Scan.Root != "":
		return rules.Expand(a.Env, a.Config.Scan.Root)
	case sys.Sandboxed(a.Env):
		return a.Env.Home()
	}
	if st, err := os.Stat(dataVolume); err == nil && st.IsDir() {
		return dataVolume
	}
	return "/"
}

func (a *App) statePath(name string) string { return filepath.Join(sys.StateDir(a.Env), name) }
