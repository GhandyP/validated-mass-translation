// Command validated-mass-translation translates a batch of text segments with a
// schema-constrained Google ADK agent and validates the result in pure Go: every
// input segment must come back exactly once, in order, under the requested target
// language, with its placeholders, markup, glossary names, and numbers preserved.
//
// The CLI is added by a later task; this entry point exists so the standalone
// module builds and tests offline while the package work lands.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "validated-mass-translation: CLI not implemented yet")
	os.Exit(2)
}
