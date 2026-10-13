package main

import (
	"context"
	"log"
	"os"

	"github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper"
)

func main() {
	if err := adkhelper.Run(
		context.Background(),
		os.Args[1:],
		"case_10_mass_translation",
		"A text-only starting point for mass translation.",
		"Translate the supplied text into the requested target language. Preserve placeholders, markup, names, and numbers; flag uncertainty instead of inventing context.",
	); err != nil {
		log.Fatal(err)
	}
}
