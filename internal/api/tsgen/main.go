// Command tsgen writes ui/src/api/types.gen.ts from the API types.
package main

import (
	"bytes"
	"flag"
	"log"
	"os"

	"github.com/doguyilmaz/ebb/internal/api"
)

func main() {
	out := flag.String("out", "", "output file")
	flag.Parse()
	var buf bytes.Buffer
	if err := api.GenerateTS(&buf); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
}
