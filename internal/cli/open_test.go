package cli

import "testing"

const hostedOrigin = "https://ebb.apps.doguyilmaz.com"

func TestWindowOrigin(t *testing.T) {
	allowed := []string{hostedOrigin, "https://mine.example"}
	cases := []struct {
		name    string
		allowed []string
		asked   string
		safari  bool
		want    string
		err     bool
	}{
		{"the first allowed page", allowed, "", false, hostedOrigin, false},
		{"Safari gets ebb's own window", allowed, "", true, "", false},
		{"no page allowed", nil, "", false, "", false},
		{"an allowed page by name", allowed, "https://mine.example/", false, "https://mine.example", false},
		{"by name in Safari too", allowed, "https://mine.example", true, "https://mine.example", false},
		{"a page that is not allowed", allowed, "https://evil.example", false, "", true},
	}
	for _, c := range cases {
		got, err := windowOrigin(c.allowed, c.asked, c.safari)
		if got != c.want || (err != nil) != c.err {
			t.Errorf("%s: %q, %v", c.name, got, err)
		}
	}
}

func TestDefaultBrowser(t *testing.T) {
	chrome := `{"LSHandlers":[{"LSHandlerURLScheme":"mailto","LSHandlerRoleAll":"com.apple.mail"},{"LSHandlerURLScheme":"https","LSHandlerRoleAll":"com.google.chrome"}]}`
	if got := httpsHandler([]byte(chrome)); got != "com.google.chrome" || safari(got) {
		t.Fatalf("chrome: %q", got)
	}
	if got := httpsHandler([]byte(`{"LSHandlers":[]}`)); !safari(got) {
		t.Fatalf("no choice made should be Safari: %q", got)
	}
	if !safari(httpsHandler([]byte("not json"))) || !safari("com.apple.SafariTechnologyPreview") {
		t.Fatal("unknown and Safari Technology Preview should get ebb's own window")
	}
}
