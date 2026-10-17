// Package plist decodes property lists by converting them to JSON with
// plutil, which reads XML and binary plists alike.
package plist

import (
	"context"
	"encoding/json"
	"os"

	"github.com/doguyilmaz/ebb/internal/sys"
)

func DecodeFile(ctx context.Context, env sys.Env, path string, v any) error {
	out, err := env.Run(ctx, "", "plutil", "-convert", "json", "-o", "-", path)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, v)
}

// JSON converts an in-memory plist to JSON.
func JSON(ctx context.Context, env sys.Env, data []byte) ([]byte, error) {
	f, err := os.CreateTemp("", "ebb-*.plist")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return env.Run(ctx, "", "plutil", "-convert", "json", "-o", "-", f.Name())
}
