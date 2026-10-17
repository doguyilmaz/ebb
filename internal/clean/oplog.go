package clean

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Op is one action carried out, kept in an append-only log.
type Op struct {
	ID       string    `json:"id"`
	PlanID   string    `json:"planId"`
	Path     string    `json:"path"`
	Action   string    `json:"action"`
	Time     time.Time `json:"time"`
	Bytes    uint64    `json:"bytes"`
	Result   string    `json:"result"` // done | failed | refused
	Error    string    `json:"error,omitempty"`
	TrashURL string    `json:"trashUrl,omitempty"`
}

type OpLog struct {
	mu   sync.Mutex
	path string
}

func OpenOpLog(stateDir string) *OpLog { return &OpLog{path: filepath.Join(stateDir, "oplog.jsonl")} }

func (l *OpLog) Append(op Op) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(op)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// List returns up to limit ops, newest first.
func (l *OpLog) List(limit int) ([]Op, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Op{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ops []Op
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var op Op
		if json.Unmarshal(sc.Bytes(), &op) == nil {
			ops = append(ops, op)
		}
	}
	slices.Reverse(ops)
	if limit > 0 && len(ops) > limit {
		ops = ops[:limit]
	}
	return ops, sc.Err()
}
