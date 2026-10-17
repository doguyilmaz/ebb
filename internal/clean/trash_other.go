//go:build !darwin

package clean

import (
	"path/filepath"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func systemTrash(env sys.Env) Trasher { return folderTrash{filepath.Join(env.Home(), ".Trash")} }
