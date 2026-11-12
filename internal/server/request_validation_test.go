package server

import "testing"

func TestChooseFolderRejectsUnknownPurpose(t *testing.T) {
	c := newClient(t)
	if status, body := c.post("/api/v1/choose-folder", "", true, `{"purpose":"unknown"}`); status != 400 {
		t.Fatalf("unknown chooser purpose: %d %v", status, body)
	}
}
