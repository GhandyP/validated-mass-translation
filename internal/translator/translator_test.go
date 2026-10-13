package translator_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
	"github.com/GhandyP/validated-mass-translation/internal/translator"
)

func input(segments ...batch.Segment) batch.TranslationBatch {
	return batch.TranslationBatch{SourceLanguage: "en", TargetLanguage: "es", Segments: segments}
}

func TestFakeAnswersEverySegmentInInputOrder(t *testing.T) {
	in := input(
		batch.Segment{ID: "cart.title", Text: "Add %s to the cart"},
		batch.Segment{ID: "spacer", Text: "   "},
	)

	got, err := translator.Fake{ByID: map[string]string{"cart.title": "Agregá %s al carrito"}}.Translate(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Translate() = %v, want nil", err)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("Translate() returned %d segments, want 2", len(got.Segments))
	}
	if got.Segments[0].ID != "cart.title" || got.Segments[0].Text != "Agregá %s al carrito" {
		t.Fatalf("first segment = %+v, want the mapped translation", got.Segments[0])
	}
	if got.Segments[1].ID != "spacer" || got.Segments[1].Text != "" {
		t.Fatalf("second segment = %+v, want an empty translation of an unmapped id", got.Segments[1])
	}
}

func TestFakeReturnsItsFailure(t *testing.T) {
	failure := errors.New("the model is unreachable")

	_, err := translator.Fake{Fail: failure}.Translate(context.Background(), input(batch.Segment{ID: "a"}), nil)
	if !errors.Is(err, failure) {
		t.Fatalf("Translate() = %v, want %v", err, failure)
	}
}

func TestFakeIsUsableWithoutAMapping(t *testing.T) {
	got, err := translator.Fake{}.Translate(context.Background(), input(batch.Segment{ID: "a", Text: "Ready"}), nil)
	if err != nil {
		t.Fatalf("Translate() = %v, want nil", err)
	}
	if len(got.Segments) != 1 || got.Segments[0].Text != "" {
		t.Fatalf("Translate() = %+v, want one segment with empty text", got)
	}
}

func TestFakeOutputPassesTheTranslationRules(t *testing.T) {
	in := input(batch.Segment{ID: "promo", Text: "Save <b>10%</b>"})

	candidate, err := translator.Fake{ByID: map[string]string{"promo": "Ahorrá <b>10%</b>"}}.Translate(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Translate() = %v, want nil", err)
	}
	if _, err := translation.Build(in, candidate, nil); err != nil {
		t.Fatalf("Build() = %v, want nil: the fake must produce a rounded candidate", err)
	}
}

func TestNewGeminiFailsClosedWithoutAnAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	model, err := translator.NewGemini(context.Background(), translator.Options{})
	if !errors.Is(err, translator.ErrModelUnavailable) {
		t.Fatalf("NewGemini() error = %v, want an error wrapping ErrModelUnavailable", err)
	}
	if model != nil {
		t.Fatalf("NewGemini() = %v, want no translator", model)
	}
}

func TestLookupGlossaryReportsProtectedTerms(t *testing.T) {
	keep := []string{"Acme", "Northwind"}

	tests := []struct {
		name          string
		term          string
		wantProtected bool
	}{
		{name: "exact term", term: "Acme", wantProtected: true},
		{name: "case insensitive", term: "acme", wantProtected: true},
		{name: "padded term", term: "  Northwind ", wantProtected: true},
		{name: "absent term", term: "Contoso", wantProtected: false},
		{name: "blank term", term: "   ", wantProtected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := translator.LookupGlossary(keep, test.term)
			if got.Protected != test.wantProtected {
				t.Fatalf("LookupGlossary(%q) protected = %v, want %v", test.term, got.Protected, test.wantProtected)
			}
			if got.Protected && got.Note == "" {
				t.Error("a protected term needs a note telling the caller what to do with it")
			}
			if !got.Protected && got.Note != "" {
				t.Errorf("an unprotected term carries the note %q, want none", got.Note)
			}
		})
	}
}

func TestLookupGlossaryWithNoGlossary(t *testing.T) {
	got := translator.LookupGlossary(nil, "Acme")
	if got.Protected {
		t.Fatalf("LookupGlossary() protected = true with an empty glossary")
	}
	if got.Term != "Acme" {
		t.Fatalf("LookupGlossary() term = %q, want the term that was asked about", got.Term)
	}
}

func TestGlossaryToolIsDeclaredToTheAgent(t *testing.T) {
	tool, err := translator.GlossaryTool([]string{"Acme"})
	if err != nil {
		t.Fatalf("GlossaryTool() = %v, want nil", err)
	}
	if tool.Name() != translator.GlossaryToolName {
		t.Fatalf("tool name = %q, want %q", tool.Name(), translator.GlossaryToolName)
	}
	if !strings.Contains(strings.ToLower(tool.Description()), "glossary") {
		t.Fatalf("tool description = %q, want it to explain the glossary lookup", tool.Description())
	}
	if tool.IsLongRunning() {
		t.Error("the glossary lookup is a local map read, not a long-running operation")
	}
}
