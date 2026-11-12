package cli

import "testing"

func TestNeedsRefreshOnlyForAnOlderService(t *testing.T) {
	for _, tc := range []struct {
		name       string
		running    string
		current    string
		defCurrent bool
		want       bool
	}{
		{name: "older service", running: "v0.1.7", current: "v0.1.8", defCurrent: true, want: true},
		{name: "newer service", running: "v0.1.9", current: "v0.1.8", defCurrent: true, want: false},
		{name: "definition changed at same version", running: "v0.1.8", current: "v0.1.8", defCurrent: false, want: true},
		{name: "definition changed but service is newer", running: "v0.1.9", current: "v0.1.8", defCurrent: false, want: false},
		{name: "development build keeps definition refresh", running: "v0.1.8", current: "dev", defCurrent: false, want: true},
		{name: "unknown mismatch does not downgrade", running: "nightly", current: "v0.1.8", defCurrent: false, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsRefresh(tc.running, tc.current, tc.defCurrent); got != tc.want {
				t.Fatalf("needsRefresh(%q, %q, %t) = %t, want %t", tc.running, tc.current, tc.defCurrent, got, tc.want)
			}
		})
	}
}
