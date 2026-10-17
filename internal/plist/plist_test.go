package plist

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func TestJSONConvertsXMLPlist(t *testing.T) {
	if _, err := sys.Real().LookPath("plutil"); err != nil {
		t.Skip("plutil unavailable")
	}
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>WorkspacePath</key><string>/p/App.xcodeproj</string></dict></plist>`)
	out, err := JSON(context.Background(), sys.Real(), xml)
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ WorkspacePath string }
	if err := json.Unmarshal(out, &v); err != nil || v.WorkspacePath != "/p/App.xcodeproj" {
		t.Fatalf("%s %v", out, err)
	}
}
