package clean

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/doguyilmaz/ebb/internal/sys"
)

// Undo puts a trashed item back where it was.
func Undo(env sys.Env, l *OpLog, opID string) error {
	ops, err := l.List(0)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.ID != opID {
			continue
		}
		if op.Action != "trash" || op.Result != "done" || op.TrashURL == "" {
			return errors.New("only items moved to the Trash can be put back")
		}
		if _, err := os.Lstat(op.Path); err == nil {
			return fmt.Errorf("something is already at %s", op.Path)
		}
		if _, err := os.Lstat(op.TrashURL); err != nil {
			return errors.New("the item is no longer in the Trash")
		}
		if err := os.MkdirAll(filepath.Dir(op.Path), 0o755); err != nil {
			return err
		}
		if err := os.Rename(op.TrashURL, op.Path); err != nil {
			return err
		}
		return l.Append(Op{ID: "undo-" + op.ID, PlanID: op.PlanID, Path: op.Path, Action: "undo", Time: env.Now(), Bytes: op.Bytes, Result: "done"})
	}
	return fmt.Errorf("no operation %q", opID)
}
