// Package volume accounts for every byte the disk reports as used: scanned
// files, the sealed system volume, other APFS volumes, snapshots, purgeable
// space, and what the scan could not see.
package volume

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/plist"
	"github.com/doguyilmaz/ebb/internal/sys"
)

type VolumeUse struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Device string `json:"device"`
	Used   uint64 `json:"used"`
}

type Snapshot struct {
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

type Breakdown struct {
	Capacity       uint64      `json:"capacity"`
	Used           uint64      `json:"used"`
	Free           uint64      `json:"free"`
	DataUsed       uint64      `json:"dataUsed"`
	SystemUsed     uint64      `json:"systemUsed"`
	Others         []VolumeUse `json:"others,omitempty"`
	Purgeable      uint64      `json:"purgeable"`
	Snapshots      []Snapshot  `json:"snapshots,omitempty"`
	Scanned        uint64      `json:"scanned"`
	NotVisible     uint64      `json:"notVisible"`
	FullDiskAccess bool        `json:"fullDiskAccess"`
}

// Inspect fills what the machine can tell about the volume holding dataRoot.
// Every external tool is optional: statfs alone still gives capacity and use.
func Inspect(ctx context.Context, env sys.Env, dataRoot string) (Breakdown, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dataRoot, &st); err != nil {
		return Breakdown{}, err
	}
	b := fromStatfs(&st)
	if raw, err := env.Run(ctx, "", "diskutil", "apfs", "list", "-plist"); err == nil {
		if js, err := plist.JSON(ctx, env, raw); err == nil {
			_ = applyAPFS(&b, js, device(&st))
		}
	}
	if out, err := env.Run(ctx, "", "tmutil", "listlocalsnapshots", "/"); err == nil {
		b.Snapshots = parseSnapshots(out)
	}
	if avail, important, ok := capacity(dataRoot); ok && important > avail {
		b.Purgeable = important - avail
	}
	b.FullDiskAccess = HasFullDiskAccess(env.Home())
	return b, nil
}

// Attribute records how much the scan saw; the rest of the data volume's use
// is "not visible" (permissions, system-only folders).
func (b *Breakdown) Attribute(scanned uint64) {
	b.Scanned = scanned
	b.NotVisible = 0
	if b.DataUsed > scanned {
		b.NotVisible = b.DataUsed - scanned
	}
}

func parseSnapshots(out []byte) []Snapshot {
	var snaps []Snapshot
	for line := range strings.Lines(string(out)) {
		name := strings.TrimSpace(line)
		stamp, ok := strings.CutPrefix(name, "com.apple.TimeMachine.")
		if !ok {
			continue
		}
		stamp = strings.TrimSuffix(stamp, ".local")
		d, err := time.ParseInLocation("2006-01-02-150405", stamp, time.Local)
		if err != nil {
			continue
		}
		snaps = append(snaps, Snapshot{Name: name, Date: d})
	}
	return snaps
}

// HasFullDiskAccess probes a TCC-protected file; only a process with Full
// Disk Access can open it.
func HasFullDiskAccess(home string) bool {
	f, err := os.Open(filepath.Join(home, "Library", "Application Support", "com.apple.TCC", "TCC.db"))
	if err != nil {
		return false
	}
	f.Close()
	return true
}
