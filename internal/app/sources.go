package app

import (
	"github.com/doguyilmaz/ebb/internal/sources"
	"github.com/doguyilmaz/ebb/internal/sys"
)

// Sources splits the last scan into place → owner → kind.
func (a *App) Sources() (sources.Flows, error) {
	r, err := a.Last()
	if err != nil {
		return sources.Flows{}, err
	}
	return sources.Build(r.Tree, a.Links, a.Env.Home(), a.View(r).Findings), nil
}

// findPackages is sources.FindPackages; a variable so a test can hold an
// answer back until a cleanup is done.
var findPackages = sources.FindPackages

// Packages groups the node_modules packages of every project and Bun's
// cache; it reads package.json files, so the answer is kept per scan.
func (a *App) Packages() (sources.Packages, error) {
	r, err := a.Last()
	if err != nil {
		return sources.Packages{}, err
	}
	a.mu.Lock()
	if a.packages.scan == r.Meta.ID && a.packages.scan != "" {
		p := a.packages.result
		a.mu.Unlock()
		return p, nil
	}
	dropped := a.packages.dropped
	a.mu.Unlock()
	p := findPackages(r.Tree, a.Links, a.Env.Home(), func(p string) ([]byte, error) { return sys.ReadSmall(p, 1<<20) }, 200)
	a.mu.Lock()
	// A cleanup that ended meanwhile kept the scan's ID, not these packages.
	if a.packages.dropped == dropped {
		a.packages.scan, a.packages.result = r.Meta.ID, p
	}
	a.mu.Unlock()
	return p, nil
}

// dropPackages forgets the packages of a scan that changed but kept its ID.
// The caller holds a.mu.
func (a *App) dropPackages() {
	a.packages.scan, a.packages.dropped = "", a.packages.dropped+1
}
