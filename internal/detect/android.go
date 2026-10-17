package detect

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/doguyilmaz/ebb/internal/finding"
)

type android struct{}

func (android) ID() string { return "android" }
func (android) Pass() int  { return 1 }

func (android) Detect(_ context.Context, c *Context) ([]finding.Finding, error) {
	var fs []finding.Finding
	referenced := map[string]bool{}
	avds := finding.Finding{RuleID: "android.avds", Title: "Android emulators unused for 30+ days", Category: "sdk", Owner: "Android Studio",
		Safety:  finding.Caution,
		Why:     "Each emulator keeps its own disk images with installed apps and data.",
		Impact:  "Apps and data inside those emulators are lost; create new ones in Android Studio's Device Manager.",
		Actions: []finding.Action{{Kind: "delete", Quit: []string{"qemu-system-aarch64", "qemu-system-x86_64"}}}}
	if dir, ok := c.node(c.home(".android/avd")); ok {
		for _, id := range c.Tree.Children(dir) {
			n := c.Tree.Node(id)
			name, ok := strings.CutSuffix(n.Name, ".avd")
			if !ok {
				continue
			}
			avdPath := c.Tree.Path(id)
			if img := sysdir(c, filepath.Join(avdPath, "config.ini")); img != "" {
				referenced[strings.Trim(img, "/")] = true
			}
			last := time.Unix(n.MaxMtime, 0)
			if c.Now.Sub(last) < c.StaleAfter {
				continue
			}
			avds.Targets = append(avds.Targets, c.target(id), finding.Target{Path: c.Links.Display(filepath.Join(filepath.Dir(avdPath), name+".ini"))})
			avds.Evidence = append(avds.Evidence, fmt.Sprintf("%s: last used %s", name, daysAgo(c.Now, last)))
		}
	}
	if len(avds.Targets) > 0 {
		fs = append(fs, finish(avds))
	}
	images := finding.Finding{RuleID: "android.images", Title: "Unused Android system images", Category: "sdk", Owner: "Android Studio",
		Safety:  finding.Regenerable,
		Why:     "System images are downloaded per API level and ABI; no remaining emulator uses these.",
		Impact:  "Android Studio downloads an image again when you create an emulator that needs it.",
		Actions: []finding.Action{{Kind: "delete"}}}
	sdk := c.home("Library/Android/sdk")
	if root, ok := c.node(filepath.Join(sdk, "system-images")); ok {
		for _, api := range c.Tree.Children(root) {
			for _, tag := range c.Tree.Children(api) {
				for _, abi := range c.Tree.Children(tag) {
					rel := strings.TrimPrefix(c.Tree.Path(abi), c.Links.Real(sdk)+"/")
					if !referenced[rel] {
						images.Targets = append(images.Targets, c.target(abi))
						images.Evidence = append(images.Evidence, rel)
					}
				}
			}
		}
	}
	if len(images.Targets) > 0 {
		fs = append(fs, finish(images))
	}
	return fs, nil
}

// sysdir reads image.sysdir.1 from an AVD's config.ini.
func sysdir(c *Context, path string) string {
	b, err := c.readFile(path)
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok && strings.TrimSpace(k) == "image.sysdir.1" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
