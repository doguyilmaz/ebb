package app

import (
	"context"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/apps"
)

// Apps lists the apps the last scan saw, biggest first, each with what it
// keeps in the Library.
func (a *App) Apps(ctx context.Context) (api.AppsResponse, error) {
	r, err := a.Last()
	if err != nil {
		return api.AppsResponse{}, err
	}
	list, _ := apps.Source{Env: a.Env, Tree: r.Tree, Links: a.Links}.Installed(ctx)
	out := api.AppsResponse{Apps: list}
	if list == nil {
		out.Apps = []apps.App{}
		if _, ok := r.Tree.Lookup(a.Links.Real("/Applications")); !ok {
			out.Note = "The last scan did not include Applications: scan the whole disk to see your apps."
		}
	}
	return out, nil
}
