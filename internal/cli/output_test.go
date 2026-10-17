package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintTableStripsControlCharacters(t *testing.T) {
	var buf bytes.Buffer
	printTable(&buf, []string{"PATH"}, [][]string{{"/x/\x1b[31mred\x1b[0m\nnext"}})
	if strings.ContainsAny(buf.String(), "\x1b") || strings.Count(buf.String(), "\n") != 2 {
		t.Fatalf("%q", buf.String())
	}
}
