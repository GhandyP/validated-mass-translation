package translator_test

import (
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/translator"
)

func TestBatchPromptNamesTheLanguagesAndEverySegment(t *testing.T) {
	in := batch.TranslationBatch{
		SourceLanguage: "en",
		TargetLanguage: "es",
		Segments: []batch.Segment{
			{ID: "cart.title", Text: "Add %s to the cart"},
			{ID: "spacer", Text: "   "},
		},
	}

	got := translator.BatchPrompt(in, []string{"Acme"})

	for _, want := range []string{"\"en\"", "\"es\"", "cart.title", "Add %s to the cart", "spacer", "Acme"} {
		if !strings.Contains(got, want) {
			t.Errorf("BatchPrompt() does not contain %q", want)
		}
	}
}

func TestBatchPromptQuotesSegmentTextSoWhitespaceIsVisible(t *testing.T) {
	in := batch.TranslationBatch{
		SourceLanguage: "en",
		TargetLanguage: "es",
		Segments:       []batch.Segment{{ID: "spacer", Text: " \t "}},
	}

	got := translator.BatchPrompt(in, nil)

	if !strings.Contains(got, "{\"id\":\"spacer\",\"text\":\" \\t \"}") {
		t.Fatalf("BatchPrompt() =\n%s\nwant the segment serialized so whitespace survives", got)
	}
}

func TestInstructionRequiresTheTargetLanguageAndThePreservationRules(t *testing.T) {
	in := batch.TranslationBatch{SourceLanguage: "en", TargetLanguage: "es"}

	got := translator.Instruction(in)

	for _, want := range []string{"\"es\"", "same order", "placeholder", "character for character", "number", "glossary"} {
		if !strings.Contains(got, want) {
			t.Errorf("Instruction() does not contain %q", want)
		}
	}
	if strings.Contains(got, "\"fr\"") {
		t.Error("Instruction() names a language the caller never asked for")
	}
}
