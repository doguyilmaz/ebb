package sys

import "testing"

func TestResponsible(t *testing.T) {
	cases := []struct {
		name      string
		ppid      int
		exe, term string
		want      Responsibility
	}{
		{"the service from the app", 1, "/Applications/ebb.app/Contents/MacOS/ebb", "", Responsibility{Kind: "app", Path: "/Applications/ebb.app"}},
		{"the service from a bare binary", 1, "/opt/homebrew/Caskroom/ebb/0.1.4/ebb", "", Responsibility{Kind: "binary", Path: "/opt/homebrew/Caskroom/ebb/0.1.4/ebb"}},
		{"a known terminal", 4242, "/Applications/ebb.app/Contents/MacOS/ebb", "iTerm.app", Responsibility{Kind: "terminal", Name: "iTerm"}},
		{"an unknown terminal", 4242, "/opt/homebrew/bin/ebb", "", Responsibility{Kind: "terminal"}},
	}
	for _, c := range cases {
		if got := responsible(c.ppid, c.exe, c.term); got != c.want {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
}
