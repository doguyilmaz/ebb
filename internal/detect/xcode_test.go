package detect

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/sys"
)

func runner(t *testing.T, outputs map[string]string) func(dir, name string, args ...string) ([]byte, error) {
	return func(dir, name string, args ...string) ([]byte, error) {
		key := name + " " + strings.Join(args, " ")
		for k, v := range outputs {
			if strings.HasPrefix(key, k) {
				if strings.HasPrefix(v, "@") {
					return os.ReadFile(v[1:])
				}
				return []byte(v), nil
			}
		}
		return nil, errors.New("not found: " + key)
	}
}

func paths(f *finding.Finding) []string {
	var out []string
	for _, t := range f.Targets {
		out = append(out, t.Path)
	}
	slices.Sort(out)
	return out
}

func simContext(t *testing.T) *Context {
	tr := home(t,
		"Library/Developer/CoreSimulator/Devices/AAAA/data/x=900",
		"Library/Developer/CoreSimulator/Devices/BBBB/data/x=400",
		"Library/Developer/CoreSimulator/Devices/CCCC/data/x=300",
		"Library/Developer/CoreSimulator/Devices/DDDD/data/x=200")
	c := ctxFor(t, tr)
	c.Env.(*sys.Fake).RunFn = runner(t, map[string]string{
		"xcrun simctl list devices -j": "@testdata/simctl-devices.json",
		"xcrun simctl runtime list -j": "@testdata/simctl-runtimes.json",
	})
	return c
}

func TestSimulatorsUnavailableAndUnused(t *testing.T) {
	fs, err := simulators{}.Detect(context.Background(), simContext(t))
	if err != nil {
		t.Fatal(err)
	}
	un := byRule(fs, "simulators.unavailable")
	if un == nil || !slices.Equal(paths(un), []string{"/h/Library/Developer/CoreSimulator/Devices/DDDD"}) || un.Bytes != 200<<20 {
		t.Fatalf("unavailable %+v", un)
	}
	if !slices.Equal(un.Targets[0].Command, []string{"xcrun", "simctl", "delete", "DDDD"}) {
		t.Fatalf("command %v", un.Targets[0].Command)
	}
	old := byRule(fs, "simulators.unused")
	if old == nil || !slices.Equal(paths(old), []string{"/h/Library/Developer/CoreSimulator/Devices/BBBB"}) || old.Safety != finding.Caution {
		t.Fatalf("unused %+v", old)
	}
	if !strings.Contains(strings.Join(old.Evidence, " "), "iPad Air") {
		t.Fatalf("evidence %v", old.Evidence)
	}
}

func TestSimulatorsNeverTargetsBooted(t *testing.T) {
	fs, _ := simulators{}.Detect(context.Background(), simContext(t))
	for _, f := range fs {
		for _, p := range paths(&f) {
			if strings.HasSuffix(p, "AAAA") {
				t.Fatalf("booted simulator offered for deletion in %s", f.RuleID)
			}
		}
	}
}

func TestSimulatorRuntimesOlderOnly(t *testing.T) {
	fs, _ := simulators{}.Detect(context.Background(), simContext(t))
	rt := byRule(fs, "simulators.runtimes")
	if rt == nil || len(rt.Targets) != 1 || rt.Targets[0].Bytes != 7000000000 {
		t.Fatalf("runtimes %+v", rt)
	}
	if !slices.Equal(rt.Targets[0].Command, []string{"xcrun", "simctl", "runtime", "delete", "R170"}) {
		t.Fatalf("command %v", rt.Targets[0].Command)
	}
	if rt.Notes == "" {
		t.Fatal("runtime mount note missing")
	}
}

func TestDerivedDataOrphaned(t *testing.T) {
	tr := home(t,
		"Library/Developer/Xcode/DerivedData/App-abc/Build/x=300",
		"Library/Developer/Xcode/DerivedData/Gone-def/Build/x=200",
		"Library/Developer/Xcode/DerivedData/ModuleCache.noindex/x=100")
	c := ctxFor(t, tr)
	c.Env.(*sys.Fake).RunFn = runner(t, map[string]string{
		"plutil -convert json -o - /h/Library/Developer/Xcode/DerivedData/App-abc/info.plist":  `{"WorkspacePath":"/p/App.xcodeproj"}`,
		"plutil -convert json -o - /h/Library/Developer/Xcode/DerivedData/Gone-def/info.plist": `{"WorkspacePath":"/p/Gone.xcodeproj"}`,
	})
	c.Exists = func(p string) bool { return p == "/p/App.xcodeproj" }
	fs, _ := derivedData{}.Detect(context.Background(), c)
	orphan := byRule(fs, "deriveddata.orphaned")
	if orphan == nil || !slices.Equal(paths(orphan), []string{"/h/Library/Developer/Xcode/DerivedData/Gone-def"}) {
		t.Fatalf("orphaned %+v", orphan)
	}
	if !strings.Contains(strings.Join(orphan.Evidence, " "), "/p/Gone.xcodeproj") {
		t.Fatalf("evidence %v", orphan.Evidence)
	}
	rest := byRule(fs, "deriveddata")
	if rest == nil || len(rest.Targets) != 2 {
		t.Fatalf("rest %+v", rest)
	}
}

func TestXcodeKeepsSelectedAndNewest(t *testing.T) {
	tr := treeAt(t, "/", "Applications/Xcode-26.6.0.app/Contents/x=3000", "Applications/Xcode-16.4.app/Contents/x=2500",
		"Applications/Xcode-beta.app/Contents/x=2800", "Users/h/Library/Developer/Xcode/Archives/2026/App.xcarchive/x=500")
	c := ctxFor(t, tr)
	c.Env.(*sys.Fake).HomeDir = "/Users/h"
	c.Env.(*sys.Fake).RunFn = runner(t, map[string]string{
		"xcode-select -p": "/Applications/Xcode-beta.app/Contents/Developer\n",
		"plutil -convert json -o - /Applications/Xcode-26.6.0.app/Contents/Info.plist": `{"CFBundleShortVersionString":"26.6"}`,
		"plutil -convert json -o - /Applications/Xcode-16.4.app/Contents/Info.plist":   `{"CFBundleShortVersionString":"16.4"}`,
		"plutil -convert json -o - /Applications/Xcode-beta.app/Contents/Info.plist":   `{"CFBundleShortVersionString":"26.1"}`,
	})
	fs, _ := xcode{}.Detect(context.Background(), c)
	old := byRule(fs, "xcode.old")
	if old == nil || !slices.Equal(paths(old), []string{"/Applications/Xcode-16.4.app"}) || old.Safety != finding.Caution {
		t.Fatalf("old xcodes %+v", old)
	}
	arch := byRule(fs, "xcode.archives")
	if arch == nil || arch.Targets[0].Path != "/Users/h/Library/Developer/Xcode/Archives" || !strings.Contains(arch.Evidence[0], "1 archive") {
		t.Fatalf("archives %+v", arch)
	}
}

func TestAndroidUnusedAvdsAndUnreferencedImages(t *testing.T) {
	tr := home(t,
		".android/avd/Pixel_7.avd/userdata.img=900@1735689600",
		".android/avd/Pixel_9.avd/userdata.img=800@1792000000",
		"Library/Android/sdk/system-images/android-34/google_apis/arm64-v8a/system.img=1500",
		"Library/Android/sdk/system-images/android-33/google_apis/arm64-v8a/system.img=1400")
	c := ctxFor(t, tr)
	c.ReadFile = func(p string) ([]byte, error) {
		switch p {
		case "/h/.android/avd/Pixel_7.avd/config.ini":
			return []byte("hw.cpu.arch=arm64\nimage.sysdir.1=system-images/android-34/google_apis/arm64-v8a/\n"), nil
		case "/h/.android/avd/Pixel_9.avd/config.ini":
			return []byte("image.sysdir.1 = system-images/android-34/google_apis/arm64-v8a/\n"), nil
		}
		return nil, os.ErrNotExist
	}
	fs, _ := android{}.Detect(context.Background(), c)
	avd := byRule(fs, "android.avds")
	if avd == nil || !slices.Equal(paths(avd), []string{"/h/.android/avd/Pixel_7.avd", "/h/.android/avd/Pixel_7.ini"}) {
		t.Fatalf("avds %+v", avd)
	}
	img := byRule(fs, "android.images")
	if img == nil || !slices.Equal(paths(img), []string{"/h/Library/Android/sdk/system-images/android-33/google_apis/arm64-v8a"}) {
		t.Fatalf("images %+v", img)
	}
}

func TestDetectorsSkipWhenToolMissing(t *testing.T) {
	c := simContext(t)
	c.Env.(*sys.Fake).RunFn = func(string, string, ...string) ([]byte, error) { return nil, sys.ErrSandbox }
	fs, err := simulators{}.Detect(context.Background(), c)
	if err != nil || len(fs) != 0 {
		t.Fatalf("missing simctl must yield nothing: %v %v", fs, err)
	}
}
