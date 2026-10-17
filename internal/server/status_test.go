package server

import (
	"os"
	"testing"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func TestStatusLiveness(t *testing.T) {
	home := t.TempDir()
	env := &sys.Fake{HomeDir: home, GOOS: "darwin", Envs: map[string]string{sys.HomeEnv: home}}
	if _, ok := Live(env); ok {
		t.Fatal("no status file, no server")
	}
	if err := WriteStatus(env, Status{PID: os.Getpid(), Port: 47340, Version: "t"}); err != nil {
		t.Fatal(err)
	}
	st, ok := Live(env)
	if !ok || st.Port != 47340 || st.URL() != "http://127.0.0.1:47340" {
		t.Fatalf("%+v %v", st, ok)
	}
	WriteStatus(env, Status{PID: 999999, Port: 1})
	if _, ok := Live(env); ok {
		t.Fatal("a dead pid is no server")
	}
	RemoveStatus(env)
	if _, err := os.Stat(statusPath(env)); !os.IsNotExist(err) {
		t.Fatal("status file left behind")
	}
}
