package apps

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/walk"
)

func put(t *testing.T, p string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func bundle(t *testing.T, dir, id, name, exe string) {
	t.Helper()
	p := filepath.Join(dir, "Contents", "Info.plist")
	put(t, filepath.Join(dir, "Contents", "MacOS", exe), 2<<20)
	doc := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>` +
		`<key>CFBundleIdentifier</key><string>` + id + `</string>` +
		`<key>CFBundleName</key><string>` + name + `</string>` +
		`<key>CFBundleExecutable</key><string>` + exe + `</string>` +
		`<key>CFBundleShortVersionString</key><string>1.2</string></dict></plist>`
	if err := os.WriteFile(p, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func old(t *testing.T, p string) {
	t.Helper()
	then := time.Now().Add(-90 * 24 * time.Hour)
	if err := filepath.Walk(p, func(q string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(q, then, then)
	}); err != nil {
		t.Fatal(err)
	}
}

// fixture: two installed apps (one in a vendor folder), their Library data,
// and what two removed apps left behind.
func fixture(t *testing.T) (Source, string) {
	return fixtureWith(t, func([]string) ([]byte, error) { return nil, nil })
}

func fixtureWith(t *testing.T, spotlight func(args []string) ([]byte, error)) (Source, string) {
	t.Helper()
	home := t.TempDir()
	lib := filepath.Join(home, "Library")
	bundle(t, filepath.Join(home, "Applications", "Notely.app"), "com.example.Notely", "Notely", "Notely")
	bundle(t, filepath.Join(home, "Applications", "Tools", "Paint.app"), "org.paint.Paint", "Paint", "Paint")
	for _, p := range []string{
		"Containers/com.example.Notely/Data/blob",
		"Group Containers/ABCDE12345.com.example.Notely.shared/blob",
		"Application Support/Notely/db",
		"Caches/com.example.Notely/cache",
		"Caches/com.example.NotelyPro/cache", // another app of the vendor: not Notely's
		"Saved Application State/com.example.Notely.savedState/data",
		"Caches/org.paint.Paint/cache",
		"Containers/com.gone.Player/Data/blob",
		"Saved Application State/net.removed.Thing.savedState/data",
		"Caches/com.apple.Safari/cache",
		"Caches/Homebrew/downloads",
		"Caches/com.fresh.Recent/cache",
	} {
		put(t, filepath.Join(lib, p), 2<<20)
	}
	put(t, filepath.Join(lib, "Preferences", "com.example.Notely.plist"), 600)
	put(t, filepath.Join(lib, "LaunchAgents", "com.example.Notely.helper.plist"), 300)
	put(t, filepath.Join(lib, "Preferences", "com.example.NotelyPro.plist"), 600)
	for _, p := range []string{"Containers/com.gone.Player", "Saved Application State/net.removed.Thing.savedState", "Caches/com.apple.Safari", "Caches/Homebrew"} {
		old(t, filepath.Join(lib, p))
	}
	res, err := walk.Scan(context.Background(), walk.Options{Root: home}, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Time: time.Now(), Envs: map[string]string{sys.HomeEnv: home, sys.SandboxEnv: "1"},
		RunFn: func(_ string, name string, args ...string) ([]byte, error) {
			switch name {
			case "plutil":
				return exec.Command(name, args...).Output()
			case "mdfind":
				return spotlight(args)
			}
			return nil, sys.ErrSandbox
		}}
	return Source{Env: env, Tree: res.Tree}, home
}

func rels(home string, a App) []string {
	var out []string
	for _, t := range a.Leftovers {
		r, _ := filepath.Rel(filepath.Join(home, "Library"), t.Path)
		out = append(out, r)
	}
	slices.Sort(out)
	return out
}

func TestInstalledAppsAndTheirLeftovers(t *testing.T) {
	s, home := fixture(t)
	got, complete := s.Installed(context.Background())
	if !complete || len(got) != 2 {
		t.Fatalf("complete %v, apps %+v", complete, got)
	}
	var notely App
	for _, a := range got {
		if a.BundleID == "com.example.Notely" {
			notely = a
		}
	}
	if notely.Name != "Notely" || notely.Executable != "Notely" || notely.Version != "1.2" || notely.Bytes < 2<<20 || notely.Apple {
		t.Fatalf("%+v", notely)
	}
	want := []string{
		"Application Support/Notely", "Caches/com.example.Notely", "Containers/com.example.Notely",
		"Group Containers/ABCDE12345.com.example.Notely.shared", "LaunchAgents/com.example.Notely.helper.plist",
		"Preferences/com.example.Notely.plist", "Saved Application State/com.example.Notely.savedState",
	}
	if g := rels(home, notely); !slices.Equal(g, want) {
		t.Fatalf("leftovers\n got %q\nwant %q", g, want)
	}
	if notely.LeftoverBytes < 5*(2<<20) {
		t.Fatalf("leftover bytes %d", notely.LeftoverBytes)
	}
}

func TestOrphansAreConservative(t *testing.T) {
	s, _ := fixture(t)
	installed, _ := s.Installed(context.Background())
	var ids []string
	orphans, ok := s.Orphans(context.Background(), installed, time.Now(), 30*24*time.Hour)
	for _, o := range orphans {
		ids = append(ids, o.BundleID)
	}
	if !ok {
		t.Fatal("spotlight answered")
	}
	slices.Sort(ids)
	// Not: Apple's, a non-bundle name, the vendor of an installed app, anything touched lately.
	if want := []string{"com.gone.Player", "net.removed.Thing"}; !slices.Equal(ids, want) {
		t.Fatalf("orphans %q, want %q", ids, want)
	}
}

func TestMatching(t *testing.T) {
	cases := []struct {
		f     func(string, string, []string) bool
		entry string
		want  bool
	}{
		{byID, "com.example.notely", true},
		{byID, "com.example.notely.helper", true},
		{byID, "com.example.notelypro", false},
		{byGroup, "group.com.example.notely", true},
		{byGroup, "abcde12345.com.example.notely", true},
		{byGroup, "abcde1234.com.example.notely", false},
		{withSuffix(".plist"), "com.example.notely.plist", true},
		{withSuffix(".plist"), "com.example.notelyx.plist", false},
	}
	for _, c := range cases {
		if got := c.f(c.entry, "com.example.notely", nil); got != c.want {
			t.Errorf("%q: got %v", c.entry, got)
		}
	}
	if byID("com.foo.bar", "com.foo", nil) {
		t.Error("a two-label ID matches only itself")
	}
}

func TestSpotlightKnowsMakersOutsideApplications(t *testing.T) {
	var query []string
	var home string
	s, home := fixtureWith(t, func(args []string) ([]byte, error) {
		query = args
		return []byte("/Library/SystemExtensions/X/net.removed.agent.systemextension   kMDItemCFBundleIdentifier = net.removed.agent\n" +
			home + "/.Trash/Player.app   kMDItemCFBundleIdentifier = com.gone.Player\n"), nil
	})
	installed, _ := s.Installed(context.Background())
	orphans, ok := s.Orphans(context.Background(), installed, time.Now(), 30*24*time.Hour)
	if !ok || len(orphans) != 1 || orphans[0].BundleID != "com.gone.Player" {
		t.Fatalf("a system extension keeps its maker present, an app in the Trash does not: %+v", orphans)
	}
	if !slices.Contains(query, `kMDItemCFBundleIdentifier == "com.gone.*"c || kMDItemCFBundleIdentifier == "net.removed.*"c`) {
		t.Fatalf("one query for every maker: %q", query)
	}
	s, _ = fixtureWith(t, func([]string) ([]byte, error) { return nil, sys.ErrSandbox })
	if orphans, ok := s.Orphans(context.Background(), installed, time.Now(), 30*24*time.Hour); ok || orphans != nil {
		t.Fatal("without Spotlight nothing counts as left behind")
	}
}
