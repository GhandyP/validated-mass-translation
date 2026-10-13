package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/locale"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
	"github.com/GhandyP/validated-mass-translation/internal/translator"
)

// Exit statuses. They are part of the command's contract, and the golden tests
// assert every one of them.
const (
	statusOK               = 0
	statusModelFailure     = 1
	statusInputError       = 2
	statusValidationFailed = 3
	statusPublishRefused   = 4
)

// Deps are the collaborators a caller may inject.
type Deps struct {
	// Translator answers the batch. Nil means "build the Gemini translator from
	// the environment", which fails closed when GOOGLE_API_KEY is missing.
	Translator translator.Translator
}

const usage = `validated-mass-translation translates a batch of text segments and validates
that every protected span survived.

Usage:
  validated-mass-translation -input <path|-> [options]

Options:
  -input <path|->     batch JSON file, or - for standard input (required)
  -target-lang <tag>  require the batch to declare this target language
  -keep <term,...>    glossary terms that must be copied verbatim
  -out json|text      output format (default json)
  -locale <path>      existing locale file to diff against
  -dry-run            print the candidate locale bundle diff and write nothing
  -publish            ask to publish; without -dry-run it is refused

Exit statuses:
  0 success, 1 model failure, 2 input error, 3 validation failed,
  4 publish refused without -dry-run

Publishing a locale bundle requires a human-reviewed diff, so this program never
writes a locale file: -dry-run prints the diff for a human to review.
`

// Run executes one command and returns its exit status.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps Deps) int {
	flags := flag.NewFlagSet("validated-mass-translation", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {}

	inputPath := flags.String("input", "", "batch JSON file, or - for standard input")
	targetLanguage := flags.String("target-lang", "", "require the batch to declare this target language")
	keepTerms := flags.String("keep", "", "comma-separated glossary terms that must be copied verbatim")
	output := flags.String("out", "json", "output format: json or text")
	localePath := flags.String("locale", "", "existing locale file to diff against")
	dryRun := flags.Bool("dry-run", false, "print the candidate locale bundle diff and write nothing")
	publish := flags.Bool("publish", false, "ask to publish; without -dry-run it is refused")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return statusOK
		}
		// The flag package already explained the problem on stderr.
		fmt.Fprint(stderr, usage)
		return statusInputError
	}

	if strings.TrimSpace(*inputPath) == "" {
		fmt.Fprintln(stderr, "validated-mass-translation: -input is required")
		fmt.Fprint(stderr, usage)
		return statusInputError
	}

	in, err := readBatch(*inputPath, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "validated-mass-translation: "+err.Error())
		return statusInputError
	}

	if wanted := strings.TrimSpace(*targetLanguage); wanted != "" && wanted != in.TargetLanguage {
		fmt.Fprintf(stderr, "validated-mass-translation: the batch declares target language %q and -target-lang asks for %q\n",
			in.TargetLanguage, wanted)
		return statusInputError
	}

	format := strings.TrimSpace(*output)
	if format != "json" && format != "text" {
		fmt.Fprintf(stderr, "validated-mass-translation: -out %q is not supported; use json or text\n", format)
		return statusInputError
	}

	// Refuse before spending a model call: publishing needs a reviewed diff, and
	// this program never writes a locale file.
	if *publish && !*dryRun {
		fmt.Fprintln(stderr, "validated-mass-translation: publishing a locale bundle requires a human-reviewed diff; run with -dry-run to print it")
		return statusPublishRefused
	}

	existing, status := readExistingLocale(*localePath, stderr)
	if status != statusOK {
		return status
	}

	engine := deps.Translator
	if engine == nil {
		built, err := translator.NewGemini(ctx, translator.Options{})
		if err != nil {
			fmt.Fprintln(stderr, "validated-mass-translation: "+err.Error())
			return statusModelFailure
		}
		engine = built
	}

	keep := parseKeep(*keepTerms)

	candidate, err := engine.Translate(ctx, in, keep)
	if err != nil {
		fmt.Fprintln(stderr, "validated-mass-translation: "+err.Error())
		if errors.Is(err, batch.ErrInvalidBatch) {
			return statusInputError
		}
		return statusModelFailure
	}

	result, err := translation.Build(in, candidate, keep)
	if err != nil {
		fmt.Fprintln(stderr, "validated-mass-translation: "+err.Error())

		var invalid *translation.ValidationError
		if errors.As(err, &invalid) {
			return statusValidationFailed
		}
		return statusModelFailure
	}

	for _, warning := range result.Warnings {
		fmt.Fprintln(stderr, "warning: "+warning)
	}

	if *dryRun {
		fmt.Fprint(stdout, locale.Diff(existing, locale.Bundle(result)))
		return statusOK
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		// The translated text is content, not markup for a browser: keeping angle
		// brackets readable makes the review diff legible.
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, "validated-mass-translation: write the result: "+err.Error())
			return statusModelFailure
		}

		return statusOK
	}

	// Text output keeps one line per segment even when a translation contains a
	// line break, by quoting the text.
	for _, segment := range result.Segments {
		fmt.Fprintf(stdout, "%s: %s\n", segment.ID, strconv.Quote(segment.Text))
	}

	return statusOK
}

// readBatch loads and validates the batch from a path or from standard input.
func readBatch(path string, stdin io.Reader) (batch.TranslationBatch, error) {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return batch.TranslationBatch{}, err
		}
		defer file.Close()

		reader = file
	}

	loaded, err := batch.Load(reader)
	if err != nil {
		return batch.TranslationBatch{}, err
	}
	if err := loaded.Validate(); err != nil {
		return batch.TranslationBatch{}, err
	}

	return loaded, nil
}

// readExistingLocale loads the locale file to diff against. A missing file is a
// first publish, not an error; an unreadable or malformed file is an input error.
func readExistingLocale(path string, stderr io.Writer) (map[string]string, int) {
	if strings.TrimSpace(path) == "" {
		return nil, statusOK
	}

	loaded, err := locale.Load(path)
	switch {
	case err == nil:
		return loaded, statusOK
	case errors.Is(err, os.ErrNotExist):
		return map[string]string{}, statusOK
	default:
		fmt.Fprintln(stderr, "validated-mass-translation: "+err.Error())
		return nil, statusInputError
	}
}

// parseKeep splits the glossary flag, trimming blanks and dropping repeats, so the
// translator, the prompt, and the tool see each protected term once.
func parseKeep(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var terms []string
	for _, entry := range strings.Split(value, ",") {
		term := strings.TrimSpace(entry)
		if term == "" {
			continue
		}
		key := strings.ToLower(term)
		if _, duplicated := seen[key]; duplicated {
			continue
		}
		seen[key] = struct{}{}
		terms = append(terms, term)
	}

	return terms
}
