// Command validated-mass-translation translates a batch of text segments with a
// schema-constrained Google ADK agent and validates the result in pure Go: every
// input segment must come back exactly once, in order, under the requested target
// language, with its placeholders, markup, glossary names, and numbers preserved.
//
// The program never writes a locale file: -dry-run prints the candidate bundle
// diff, so publishing stays a human-reviewed change.
package main

import (
	"context"
	"os"

	"github.com/GhandyP/validated-mass-translation/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cli.Deps{}))
}
