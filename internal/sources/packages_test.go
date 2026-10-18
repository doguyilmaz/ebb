package sources

import (
	"os"
	"slices"
	"testing"

	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

func TestPackagesAcrossProjects(t *testing.T) {
	tr := build(t, "/h", map[string]uint64{
		"code/a/package.json":                                                          0,
		"code/a/node_modules/react/index.js":                                           20,
		"code/a/node_modules/@babel/core/lib.js":                                       30,
		"code/a/node_modules/.bin/x":                                                   1,
		"code/b/package.json":                                                          0,
		"code/b/node_modules/react/index.js":                                           20,
		"code/b/node_modules/react/node_modules/scheduler/x.js":                        5,
		"code/c/package.json":                                                          0,
		"code/c/node_modules/.pnpm/react@19.2.0/node_modules/react/x.js":               20,
		"code/c/node_modules/.pnpm/@types+node@22.1.0_x/node_modules/@types/node/x.js": 7,
		".bun/install/cache/react@19.2.0@@@1/x":                                        4,
		".bun/install/cache/react@18.3.1@@@1/x":                                        4,
		".bun/install/cache/@types/node@22.1.0@@@1/x":                                  2,
		".bun/install/cache/@G@abc/x":                                                  9,
		"Library/x/node_modules/react/index.js":                                        50,
	})
	for _, p := range []string{"/h/code/a", "/h/code/b", "/h/code/c"} {
		id, _ := tr.Lookup(p)
		tr.Write(func(w *tree.Writer) { w.SetMarkers(id, tree.MarkerBit("package.json")) })
	}
	read := func(p string) ([]byte, error) {
		switch p {
		case "/h/code/a/node_modules/react/package.json":
			return []byte(`{"name":"react","version":"19.2.0"}`), nil
		case "/h/code/b/node_modules/react/package.json":
			return []byte(`{"name":"react","version":"18.3.1"}`), nil
		}
		return nil, os.ErrNotExist
	}
	got := FindPackages(tr, volume.Firmlinks{}, "/h", read, 50)
	if got.Projects != 3 {
		t.Fatalf("projects %d", got.Projects)
	}
	var react, babel, types *Package
	for i := range got.Packages {
		switch got.Packages[i].Name {
		case "react":
			react = &got.Packages[i]
		case "@babel/core":
			babel = &got.Packages[i]
		case "@types/node":
			types = &got.Packages[i]
		}
	}
	if react == nil || react.Copies != 3 || !slices.Equal(react.Versions, []string{"18.3.1", "19.2.0"}) || len(react.Projects) != 3 {
		t.Fatalf("react %+v", react)
	}
	if babel == nil || babel.Copies != 1 || types == nil || types.Versions[0] != "22.1.0" {
		t.Fatalf("scoped: %+v %+v", babel, types)
	}
	if len(got.Cache) != 2 || got.Cache[0].Name != "react" || !slices.Equal(got.Cache[0].Versions, []string{"18.3.1", "19.2.0"}) || got.Cache[1].Name != "@types/node" {
		t.Fatalf("bun cache %+v", got.Cache)
	}
}
