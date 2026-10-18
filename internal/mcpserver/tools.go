package mcpserver

import (
	"context"
	"errors"

	"github.com/doguyilmaz/ebb/internal/api"
	"github.com/doguyilmaz/ebb/internal/clean"
)

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	run         func(ctx context.Context, args map[string]any) (any, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func str(desc string) map[string]any   { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any   { return map[string]any{"type": "number", "description": desc} }
func boolp(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func arr(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func getS(args map[string]any, k string) string { v, _ := args[k].(string); return v }

func getB(args map[string]any, k string, def bool) bool {
	if v, ok := args[k].(bool); ok {
		return v
	}
	return def
}

func getN(args map[string]any, k string, def, lo, hi int) int {
	v, ok := args[k].(float64)
	if !ok {
		return def
	}
	return max(lo, min(hi, int(v)))
}

func getList(args map[string]any, k string) []string {
	var out []string
	if xs, ok := args[k].([]any); ok {
		for _, x := range xs {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// brief keeps finding lists short enough for an agent's context.
func brief(fs []api.FindingView) []map[string]any {
	out := make([]map[string]any, 0, len(fs))
	for _, f := range fs {
		kind := ""
		if len(f.Actions) > 0 {
			kind = f.Actions[0].Kind
		}
		out = append(out, map[string]any{"id": f.ID, "title": f.Title, "category": f.Category, "safety": f.Safety, "bytes": f.Bytes,
			"why": f.Why, "impact": f.Impact, "locations": len(f.Targets), "action": kind})
	}
	return out
}

func (s *Server) define() []tool {
	b := s.b
	return []tool{
		{Name: "ebb_status", Description: "ebb's state: version, whether a scan runs, the last scan (root, time, files) and how many folders changed since.",
			InputSchema: obj(map[string]any{}),
			run:         func(ctx context.Context, _ map[string]any) (any, error) { return b.Status(ctx) }},
		{Name: "ebb_scan", Description: "Scan the disk (or a folder). After the first scan only folders that changed are read again, usually in seconds; full=true reads everything.",
			InputSchema: obj(map[string]any{"root": str("folder to scan; empty = the whole disk"), "wait": boolp("wait until it finishes (default true)"), "full": boolp("read every folder")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				return b.Scan(ctx, api.ScanRequest{Root: getS(args, "root"), Full: getB(args, "full", false)}, getB(args, "wait", true))
			}},
		{Name: "ebb_overview", Description: "Where the disk's space is (macOS, apps, what can be cleaned, everything else, not visible, free) and the largest findings.",
			InputSchema: obj(map[string]any{}),
			run: func(ctx context.Context, _ map[string]any) (any, error) {
				v, err := b.Volume(ctx)
				if err != nil {
					return nil, err
				}
				f, err := b.Findings(ctx, "", 0)
				if err != nil {
					return nil, err
				}
				top := f.Findings[:min(8, len(f.Findings))]
				return map[string]any{"volume": v, "reclaimable": f.Reclaimable, "largestFindings": brief(top), "notes": f.Notes}, nil
			}},
		{Name: "ebb_top", Description: "The biggest items in a folder (kind=dirs, default) or the biggest files anywhere below it (kind=files), optionally only files untouched for N days.",
			InputSchema: obj(map[string]any{"path": str("folder; empty = the scanned root"), "n": num("how many (default 20)"), "kind": str("dirs or files"), "olderThanDays": num("files only: untouched for at least this many days")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				n := getN(args, "n", 20, 1, 200)
				if getS(args, "kind") == "files" {
					return b.Files(ctx, getS(args, "path"), 1<<20, getN(args, "olderThanDays", 0, 0, 36500), n)
				}
				return b.Children(ctx, getS(args, "path"), n)
			}},
		{Name: "ebb_search", Description: "Find folders and large files by name across the scan: text matches anywhere in the name, *.dmg style globs match whole names.",
			InputSchema: obj(map[string]any{"query": str("name or glob")}, "query"),
			run:         func(ctx context.Context, args map[string]any) (any, error) { return b.Search(ctx, getS(args, "query")) }},
		{Name: "ebb_findings", Description: "What ebb found that takes real space, each with why it is there, what removing it costs, how safe it is (regenerable, caution, user-data, system) and its id for planning.",
			InputSchema: obj(map[string]any{"category": str("optional: package-cache, build-artifact, simulator, sdk, stale-project, app-cache, logs, installer, large-file, trash, leftover, duplicate"), "minBytes": num("only findings at least this big")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				f, err := b.Findings(ctx, getS(args, "category"), uint64(getN(args, "minBytes", 0, 0, 1<<62)))
				if err != nil {
					return nil, err
				}
				return map[string]any{"reclaimable": f.Reclaimable, "findings": brief(f.Findings)}, nil
			}},
		{Name: "ebb_explain", Description: "Everything ebb knows about one path: sizes (allocated, reclaimable), files, last change, the findings covering it with why and impact, and what is biggest inside.",
			InputSchema: obj(map[string]any{"path": str("absolute path inside the scan")}, "path"),
			run:         func(ctx context.Context, args map[string]any) (any, error) { return b.Explain(ctx, getS(args, "path")) }},
		{Name: "ebb_presets", Description: "Ready-made cleanups (for example every cache that rebuilds itself) with what each would free now.",
			InputSchema: obj(map[string]any{}),
			run:         func(ctx context.Context, _ map[string]any) (any, error) { return b.Presets(ctx) }},
		{Name: "ebb_apps", Description: "Installed apps, biggest first, with the size of the app and of what it keeps in the Library (containers, caches, preferences, saved state, launch agents). Uninstall one with ebb_cleanup_plan app=<name or bundle id>.",
			InputSchema: obj(map[string]any{"n": num("how many (default 30)")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				res, err := b.Apps(ctx)
				if err != nil {
					return nil, err
				}
				var out []map[string]any
				for _, a := range res.Apps[:min(getN(args, "n", 30, 1, 500), len(res.Apps))] {
					out = append(out, map[string]any{"name": a.Name, "bundleId": a.BundleID, "appBytes": a.Bytes, "libraryBytes": a.LeftoverBytes,
						"libraryLocations": len(a.Leftovers), "apple": a.Apple, "installedForAllUsers": a.RootOwned})
				}
				return map[string]any{"apps": out, "note": res.Note}, nil
			}},
		{Name: "ebb_duplicates", Description: "Files with the same content in the person's own folders (default: home), found by reading files of equal size; bundles, Photos/Music libraries, ~/Library, node_modules and dot-folders are left out. Each group lists the copy to keep first. Remove extra copies with ebb_cleanup_plan paths (they go to the Trash).",
			InputSchema: obj(map[string]any{"path": str("folder to search; empty = home"), "minSize": num("smallest file in bytes (default and at least 1 MiB)"), "n": num("how many groups (default 20)")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				res, err := b.Duplicates(ctx, getS(args, "path"), uint64(getN(args, "minSize", 0, 0, 1<<50)))
				if err != nil {
					return nil, err
				}
				groups := res.Groups[:min(getN(args, "n", 20, 1, 200), len(res.Groups))]
				var out []map[string]any
				for _, g := range groups {
					var paths []string
					for _, c := range g.Copies {
						paths = append(paths, c.Path)
					}
					out = append(out, map[string]any{"size": g.Size, "extraBytes": g.Wasted, "keep": paths[0], "extras": paths[1:]})
				}
				return map[string]any{"groups": out, "extraBytes": res.Wasted, "groupCount": len(res.Groups)}, nil
			}},
		{Name: "ebb_history_diff", Description: "What grew and shrank between two scans; without ids, the newest against the one before it.",
			InputSchema: obj(map[string]any{"from": str("older snapshot id"), "to": str("newer snapshot id")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				return b.Diff(ctx, getS(args, "from"), getS(args, "to"))
			}},
		{Name: "ebb_cleanup_plan", Description: "Make a cleanup plan from finding ids, a preset or paths. Nothing is removed: the plan lists every item, its action (trash, delete, the tool's own command, or manual), size, safety and anything blocking it. Show it to the person.",
			InputSchema: obj(map[string]any{"findingIds": arr("finding ids from ebb_findings"), "preset": str("preset id from ebb_presets"),
				"paths": arr("absolute paths inside the scan; a path a finding covers gets that finding's action (which may delete it outright), any other goes to the Trash"),
				"app":   str("uninstall this app (name or bundle id from ebb_apps): the app and its Library data go to the Trash")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				req := clean.Request{FindingIDs: getList(args, "findingIds"), Preset: getS(args, "preset"), Paths: getList(args, "paths"), App: getS(args, "app"), Source: "mcp"}
				if len(req.FindingIDs) == 0 && req.Preset == "" && len(req.Paths) == 0 && req.App == "" {
					return nil, errors.New("give findingIds, a preset, paths or an app")
				}
				return b.Plan(ctx, req)
			}},
		{Name: "ebb_cleanup_execute", Description: "Run a plan. Plans of only rebuildable caches run directly; any other plan needs the person's approval first (ebb's window shows it to them, or they run ebb approve <id>) and this returns an error until then. Every item is checked again before it is touched.",
			InputSchema: obj(map[string]any{"planId": str("id from ebb_cleanup_plan")}, "planId"),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				return b.Execute(ctx, getS(args, "planId"))
			}},
		{Name: "ebb_undo", Description: "Put back something a cleanup moved to the Trash, by operation id (ops[].id in the ebb_cleanup_execute result).",
			InputSchema: obj(map[string]any{"opId": str("operation id")}, "opId"),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				if err := b.Undo(ctx, getS(args, "opId")); err != nil {
					return nil, err
				}
				return "put back", nil
			}},
		{Name: "ebb_note", Description: "Leave a note on a finding (finding:<id>) or a folder (path:<absolute path>), such as why it stays. An empty text removes the note.",
			InputSchema: obj(map[string]any{"target": str("finding:<id> or path:<absolute path>"), "text": str("the note")}, "target"),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				if err := b.Note(ctx, getS(args, "target"), getS(args, "text")); err != nil {
					return nil, err
				}
				return "saved", nil
			}},
		{Name: "ebb_report", Description: "The full report of the last scan as text (default), json or csv.",
			InputSchema: obj(map[string]any{"format": str("text, json or csv")}),
			run: func(ctx context.Context, args map[string]any) (any, error) {
				return b.Report(ctx, getS(args, "format"))
			}},
	}
}
