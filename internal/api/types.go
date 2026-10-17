// Package api holds the JSON contract of the local HTTP API. These types are
// the single source: ui/src/api/types.gen.ts is generated from them.
package api

//go:generate go run ./tsgen -out ../../ui/src/api/types.gen.ts

import (
	"reflect"

	"github.com/doguyilmaz/ebb/internal/clean"
	"github.com/doguyilmaz/ebb/internal/finding"
	"github.com/doguyilmaz/ebb/internal/history"
	"github.com/doguyilmaz/ebb/internal/rules"
	"github.com/doguyilmaz/ebb/internal/tree"
	"github.com/doguyilmaz/ebb/internal/volume"
)

type Progress struct {
	Dirs      uint64 `json:"dirs"`
	Files     uint64 `json:"files"`
	Bytes     uint64 `json:"bytes"`
	Problems  uint64 `json:"problems"`
	Path      string `json:"path"`
	ElapsedMs int64  `json:"elapsedMs"`
}

type ScanState struct {
	Running  bool      `json:"running"`
	Root     string    `json:"root,omitempty"`
	Progress *Progress `json:"progress"`
	Error    string    `json:"error,omitempty"`
}

type Status struct {
	Version        string        `json:"version"`
	Scan           ScanState     `json:"scan"`
	Last           *history.Meta `json:"last"`
	DefaultRoot    string        `json:"defaultRoot"`
	FullDiskAccess bool          `json:"fullDiskAccess"`
	AskAvailable   bool          `json:"askAvailable"`
	Sandboxed      bool          `json:"sandboxed"`
	Changes        *Changes      `json:"changes"`
}

// Changes counts folders that changed on disk since the last scan, while ebb
// watches. Lost means the next scan has to be complete; Ready that the replay
// of history since the scan is over.
type Changes struct {
	Folders int  `json:"folders"`
	Lost    bool `json:"lost"`
	Ready   bool `json:"ready"`
}

type ScanRequest struct {
	Root    string `json:"root,omitempty"`
	Threads int    `json:"threads,omitempty"`
	Gentle  bool   `json:"gentle,omitempty"`
	// Full reads every folder; otherwise only folders changed since the last scan are listed again when that is known.
	Full bool `json:"full,omitempty"`
}

type SliceResponse struct {
	Path string          `json:"path"`
	Root *tree.SliceNode `json:"root"`
}

// AgeBucket holds the bytes last changed less than Days days before the scan;
// Days is 0 for the oldest bucket.
type AgeBucket struct {
	Label string `json:"label"`
	Days  int    `json:"days"`
	Bytes uint64 `json:"bytes"`
}

type AgesResponse struct {
	Path    string      `json:"path"`
	Buckets []AgeBucket `json:"buckets"`
}

// ChooseRequest asks ebb to show the macOS folder chooser; Purpose is "scan" or "protect".
type ChooseRequest struct {
	Purpose string `json:"purpose"`
}

// ChooseResponse has the picked folder, or an empty path when the person cancelled.
type ChooseResponse struct {
	Path string `json:"path"`
}

type Item struct {
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"` // dir | file | link | other
	Alloc    uint64   `json:"alloc"`
	Bytes    uint64   `json:"bytes"` // reclaimable
	Files    uint32   `json:"files"`
	Modified int64    `json:"modified"` // unix seconds, newest in subtree
	Findings []string `json:"findings,omitempty"`
	Note     string   `json:"note,omitempty"`
}

type SmallFile struct {
	Name     string `json:"name"`
	Size     uint64 `json:"size"`
	Modified int64  `json:"modified"`
}

type ChildrenResponse struct {
	Path  string      `json:"path"`
	Items []Item      `json:"items"`
	Small []SmallFile `json:"small,omitempty"`
}

type SearchResponse struct {
	Items []Item `json:"items"`
}

type FindingView struct {
	finding.Finding
	Ignored bool `json:"ignored,omitempty"`
}

type (
	PlanRequest = clean.Request
	Plan        = clean.Plan
	PlanItem    = clean.Item
	PlanEvent   = clean.Event
	PlanResult  = clean.Result
	Op          = clean.Op
)

type NoteRequest struct {
	Key  string `json:"key"` // "finding:<id>" or "path:<path>"
	Text string `json:"text"`
}

type IgnoreRequest struct {
	FindingID string `json:"findingId"`
}

type OpsResponse struct {
	Ops []Op `json:"ops"`
}

type FindingsResponse struct {
	Findings    []FindingView `json:"findings"`
	Reclaimable uint64        `json:"reclaimable"`
	Notes       []string      `json:"notes"` // what the numbers include and leave out
}

type HistoryResponse struct {
	Snapshots []history.Meta `json:"snapshots"`
}

// DiffResponse compares two snapshots; From is null while there is no earlier
// scan of the same folder to compare with.
type DiffResponse struct {
	From    *history.Meta    `json:"from"`
	To      history.Meta     `json:"to"`
	Changes []history.Change `json:"changes"`
}

type Settings struct {
	Root      string   `json:"root"`
	Threads   int      `json:"threads"`
	StaleDays int      `json:"staleDays"`
	LargeFile string   `json:"largeFile"`
	Protect   []string `json:"protect"`
}

type PresetView struct {
	rules.Preset
	Bytes uint64 `json:"bytes"`
}

type PathRequest struct {
	Path string `json:"path"`
}

type WebAccess struct {
	Allowed []string     `json:"allowed"`
	Paired  []string     `json:"paired"`
	Pending *PendingPair `json:"pending"`
}

type PendingPair struct {
	Origin    string `json:"origin"`
	Code      string `json:"code"`
	Requested string `json:"requested"`
	Confirmed bool   `json:"confirmed"`
}

type PairRequest struct {
	Nonce string `json:"nonce"`
}

type PairResponse struct {
	Pending bool   `json:"pending,omitempty"`
	Code    string `json:"code,omitempty"`
	Expires string `json:"expires,omitempty"`
	Confirm string `json:"confirm,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
	Device  string `json:"device,omitempty"`
	Version string `json:"version,omitempty"`
}

type Health struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
	Allowed bool   `json:"allowed"`
	Device  string `json:"device,omitempty"`
	Port    int    `json:"port,omitempty"` // the port this ebb listens on, also behind a dev proxy
}

type Error struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// Server-sent event names.
const (
	EventHello           = "hello"
	EventScanProgress    = "scan.progress"
	EventScanDone        = "scan.done"
	EventFindingsChanged = "findings.changed"
	EventPlanProgress    = "plan.progress"
	EventPlanDone        = "plan.done"
	EventHistoryChanged  = "history.changed"
)

// Types lists everything the UI receives or sends, for the TS generator.
var Types = []any{
	Status{}, ScanRequest{}, SliceResponse{}, AgesResponse{}, ChooseRequest{}, ChooseResponse{}, ChildrenResponse{}, SearchResponse{}, FindingsResponse{},
	HistoryResponse{}, DiffResponse{}, Settings{}, PresetView{}, PathRequest{}, WebAccess{}, PairRequest{}, PairResponse{}, Health{},
	Error{}, volume.Breakdown{}, PlanRequest{}, Plan{}, PlanEvent{}, PlanResult{}, NoteRequest{}, IgnoreRequest{}, OpsResponse{},
}

// Names gives types from other packages their API name in TypeScript.
var Names = map[reflect.Type]string{
	reflect.TypeFor[clean.Request](): "PlanRequest",
	reflect.TypeFor[clean.Plan]():    "Plan",
	reflect.TypeFor[clean.Item]():    "PlanItem",
	reflect.TypeFor[clean.Event]():   "PlanEvent",
	reflect.TypeFor[clean.Result]():  "PlanResult",
	reflect.TypeFor[clean.Op]():      "Op",
}

var Events = []string{EventHello, EventScanProgress, EventScanDone, EventFindingsChanged, EventPlanProgress, EventPlanDone, EventHistoryChanged}
