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
	a.mu.Unlock()
	p := sources.FindPackages(r.Tree, a.Links, a.Env.Home(), func(p string) ([]byte, error) { return sys.ReadSmall(p, 1<<20) }, 200)
	a.mu.Lock()
	a.packages.scan, a.packages.result = r.Meta.ID, p
	a.mu.Unlock()
	return p, nil
}
