package clean

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/doguyilmaz/ebb/internal/sys"
	"github.com/doguyilmaz/ebb/internal/volume"
)

var (
	ErrNeedsApproval = errors.New("this plan needs approval first: ebb approve <plan> or approve it in the UI")
	ErrExpired       = errors.New("this plan expired; make a new one so it reflects the disk as it is now")
)

type Trasher interface {
	Trash(path string) (string, error)
}

type Event struct {
	Item  int    `json:"item"`
	Path  string `json:"path"`
	State string `json:"state"` // running | done | refused | failed | manual
	Bytes uint64 `json:"bytes,omitempty"`
	Err   string `json:"err,omitempty"`
}

type Result struct {
	PlanID  string `json:"planId"`
	Freed   uint64 `json:"freed"`
	Done    int    `json:"done"`
	Refused int    `json:"refused"`
	Failed  int    `json:"failed"`
	Manual  int    `json:"manual"`
	Ops     []Op   `json:"ops"`
}

type Executor struct {
	Env       sys.Env
	Log       *OpLog
	Trash     Trasher
	Guard     Guard
	Links     volume.Firmlinks
	AllowRoot bool
	euid      func() int
}

// Execute carries out an approved plan. Every item is checked again right
// before it is touched: it must still be the same file system object the
// plan saw, pass the guard, and its owner app must not be running.
func (e *Executor) Execute(ctx context.Context, p *Plan, fn func(Event)) (Result, error) {
	res := Result{PlanID: p.ID, Ops: []Op{}}
	euid := e.euid
	if euid == nil {
		euid = os.Geteuid
	}
	switch {
	case euid() == 0 && !e.AllowRoot:
		return res, errors.New("ebb does not delete as root; run it as yourself")
	case e.Env.Now().After(p.Expires):
		return res, ErrExpired
	case p.NeedsApproval && !p.Approved:
		return res, ErrNeedsApproval
	}
	if fn == nil {
		fn = func(Event) {}
	}
	for i, it := range p.Items {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		op := Op{ID: fmt.Sprintf("%s-%d", p.ID, i), PlanID: p.ID, Path: it.Path, Action: it.Action.Kind, Time: e.Env.Now(), Bytes: it.Bytes}
		if it.Action.Kind == "manual" {
			res.Manual++
			fn(Event{Item: i, Path: it.Path, State: "manual"})
			continue
		}
		fn(Event{Item: i, Path: it.Path, State: "running"})
		trashURL, err := e.run(ctx, it)
		var refused *refusal
		switch {
		case errors.As(err, &refused):
			res.Refused++
			op.Result, op.Error = "refused", err.Error()
			fn(Event{Item: i, Path: it.Path, State: "refused", Err: err.Error()})
		case err != nil:
			res.Failed++
			op.Result, op.Error = "failed", err.Error()
			fn(Event{Item: i, Path: it.Path, State: "failed", Err: err.Error()})
		default:
			res.Done++
			res.Freed += it.Bytes
			op.Result, op.TrashURL = "done", trashURL
			fn(Event{Item: i, Path: it.Path, State: "done", Bytes: it.Bytes})
		}
		if err := e.Log.Append(op); err != nil {
			return res, err
		}
		res.Ops = append(res.Ops, op)
	}
	return res, nil
}

type refusal struct{ msg string }

func (r *refusal) Error() string { return r.msg }

func refuse(format string, args ...any) error { return &refusal{fmt.Sprintf(format, args...)} }

func (e *Executor) run(ctx context.Context, it Item) (string, error) {
	if it.Blocked != "" {
		return "", refuse("%s", it.Blocked)
	}
	real := e.Links.Real(it.Path)
	if sys.Sandboxed(e.Env) && !within(canonical(real), canonical(e.Env.Home())) && it.Action.Kind != "command" {
		return "", refuse("sandboxed sessions only change files inside %s", e.Env.Home())
	}
	if it.Action.Kind != "empty-trash" {
		if err := e.Guard.Check(it.Path); err != nil {
			return "", refuse("%v", err)
		}
	}
	for _, name := range it.Action.Quit {
		if running(e.Env, name) {
			return "", refuse("Quit %s first", name)
		}
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(real))
	if err != nil {
		return "", refuse("no longer exists")
	}
	parent, err := os.OpenRoot(dir)
	if err != nil {
		return "", refuse("no longer exists")
	}
	defer parent.Close()
	name := filepath.Base(real)
	fi, err := parent.Lstat(name)
	if err != nil {
		return "", refuse("no longer exists")
	}
	if !same(fi, it) {
		return "", refuse("changed since the plan was made")
	}
	switch it.Action.Kind {
	case "delete":
		// Move the verified entry to a private name first, then check that
		// what moved is still the planned object: nothing swapped in after
		// the check above can be deleted by mistake.
		tmp := ".ebb-delete-" + newID()
		if err := parent.Rename(name, tmp); err != nil {
			return "", err
		}
		moved, err := parent.Lstat(tmp)
		if err != nil || !same(moved, it) {
			_ = parent.Rename(tmp, name)
			return "", refuse("changed while being removed")
		}
		return "", parent.RemoveAll(tmp)
	case "trash":
		if it.Action.Unload {
			// A loaded launch agent keeps running, and starting, after its file is gone.
			_, _ = e.Env.Run(ctx, "", "launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), filepath.Join(dir, name))
		}
		return e.Trash.Trash(filepath.Join(dir, name))
	case "empty-trash":
		return "", e.emptyTrash(real)
	case "command":
		cctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		if len(it.Command) == 0 {
			return "", errors.New("no command to run")
		}
		// Tools read project config from their working folder (yarnPath, a
		// bunfig cache dir), so they never run where ebb was started. bun pm
		// refuses to run outside a project, so the folder holds an empty one.
		cwd, err := os.MkdirTemp("", "ebb-run-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(cwd)
		if err := os.WriteFile(filepath.Join(cwd, "package.json"), []byte("{}\n"), 0o600); err != nil {
			return "", err
		}
		_, err = e.Env.Run(cctx, cwd, it.Command[0], it.Command[1:]...)
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if line, _, _ := strings.Cut(strings.TrimSpace(string(exit.Stderr)), "\n"); line != "" {
				return "", fmt.Errorf("%s: %s", it.Command[0], line)
			}
		}
		return "", err
	}
	return "", fmt.Errorf("unknown action %q", it.Action.Kind)
}

// emptyTrash removes the Trash's contents, each entry checked by the guard.
func (e *Executor) emptyTrash(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	des, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, de := range des {
		if err := e.Guard.Check(filepath.Join(dir, de.Name())); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := root.RemoveAll(de.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func same(fi os.FileInfo, it Item) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int64(st.Dev) == it.Dev && st.Ino == it.Ino
}
