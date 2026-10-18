package plist

import "testing"

func TestTopStrings(t *testing.T) {
	doc := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.App</string>
	<key>LSMinimumSystemVersion</key>
	<date>2026-01-01T00:00:00Z</date>
	<key>Nested</key>
	<dict>
		<key>CFBundleName</key>
		<string>not this one</string>
	</dict>
	<key>CFBundleName</key>
	<string>Example &amp; Co</string>
	<key>CFBundleShortVersionString</key>
	<string>2.1</string>
</dict>
</plist>`)
	got, err := topStrings(doc, []string{"CFBundleIdentifier", "CFBundleName", "CFBundleShortVersionString"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"CFBundleIdentifier": "com.example.App", "CFBundleName": "Example & Co", "CFBundleShortVersionString": "2.1"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q, want %q", k, got[k], v)
		}
	}
}
