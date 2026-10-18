// Package plist decodes property lists by converting them to JSON with
// plutil, which reads XML and binary plists alike.
package plist

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"slices"

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

// Strings reads top-level string values of a plist file. It goes through XML,
// which plutil writes for every plist (JSON fails on dates and data, which
// some Info.plist files hold).
func Strings(ctx context.Context, env sys.Env, path string, keys ...string) (map[string]string, error) {
	out, err := env.Run(ctx, "", "plutil", "-convert", "xml1", "-o", "-", path)
	if err != nil {
		return nil, err
	}
	return topStrings(out, keys)
}

func topStrings(data []byte, keys []string) (map[string]string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	got := map[string]string{}
	depth, key := 0, ""
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return got, nil
			}
			return got, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			// plist > dict > key|string: depth 3 is the top-level dict's children.
			if depth != 3 {
				continue
			}
			var text string
			if err := d.DecodeElement(&text, &t); err != nil {
				return got, err
			}
			depth--
			switch t.Name.Local {
			case "key":
				key = text
			case "string":
				if slices.Contains(keys, key) {
					got[key] = text
				}
				key = ""
			default:
				key = ""
			}
		case xml.EndElement:
			depth--
		}
	}
}
