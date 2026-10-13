package batch_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
)

const fullDocument = `{
  "sourceLanguage": "en",
  "targetLanguage": "es",
  "audience": "internal",
  "segments": [
    {"id": "a", "text": "Keep %s intact"},
    {"id": "b", "text": "And <b>markup</b> too"}
  ]
}`

func TestLoadReadsTheFrozenWireFormat(t *testing.T) {
	got, err := batch.Load(strings.NewReader(fullDocument))
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if got.SourceLanguage != "en" || got.TargetLanguage != "es" || got.Audience != "internal" {
		t.Fatalf("Load() languages = %q -> %q (audience %q)", got.SourceLanguage, got.TargetLanguage, got.Audience)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("Load() read %d segments, want 2", len(got.Segments))
	}
	if got.Segments[0].Text != "Keep %s intact" || got.Segments[1].Text != "And <b>markup</b> too" {
		t.Fatalf("Load() changed segment text: %q, %q", got.Segments[0].Text, got.Segments[1].Text)
	}
}

func TestLoadAcceptsADocumentWithoutAnAudience(t *testing.T) {
	got, err := batch.Load(strings.NewReader(`{"sourceLanguage":"en","targetLanguage":"es","segments":[{"id":"a","text":"x"}]}`))
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if got.Audience != "" {
		t.Fatalf("Load() audience = %q, want an empty string", got.Audience)
	}
}

func TestLoadKeepsSegmentOrderAndWhitespace(t *testing.T) {
	got, err := batch.Load(strings.NewReader(`{"sourceLanguage":"en","targetLanguage":"es","segments":[{"id":"z","text":" second "},{"id":"a","text":"first"}]}`))
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if got.Segments[0].ID != "z" || got.Segments[1].ID != "a" {
		t.Fatalf("Load() reordered segments: %q, %q", got.Segments[0].ID, got.Segments[1].ID)
	}
	if got.Segments[0].Text != " second " {
		t.Fatalf("Load() rewrote whitespace: %q", got.Segments[0].Text)
	}
}

func TestLoadRejectsMalformedDocuments(t *testing.T) {
	tests := []struct {
		name     string
		document string
		wantText string
	}{
		{name: "empty input", document: "", wantText: "empty"},
		{name: "blank input", document: "   \n\t ", wantText: "empty"},
		{name: "null document", document: "null", wantText: "null"},
		{name: "unknown field", document: `{"sourceLanguage":"en","targetLanguage":"es","segments":[],"extra":1}`, wantText: "extra"},
		{name: "not an object", document: `[]`, wantText: "decode"},
		{name: "wrong field type", document: `{"segments":"nope"}`, wantText: "decode"},
		{name: "trailing text", document: `{"sourceLanguage":"en","targetLanguage":"es","segments":[]} trailing`, wantText: "trailing"},
		{name: "two documents", document: `{"sourceLanguage":"en","targetLanguage":"es","segments":[]}{"sourceLanguage":"fr"}`, wantText: "more than one"},
		{name: "unterminated object", document: `{"sourceLanguage":"en"`, wantText: "decode"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := batch.Load(strings.NewReader(test.document))
			if !errors.Is(err, batch.ErrInvalidBatch) {
				t.Fatalf("Load(%q) = %v, want an error wrapping ErrInvalidBatch", test.document, err)
			}
			if !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("Load(%q) error = %q, want it to contain %q", test.document, err.Error(), test.wantText)
			}
		})
	}
}

func TestLoadAcceptsAnEmptyObjectThatValidationThenRejects(t *testing.T) {
	got, err := batch.Load(strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Load() = %v, want nil: shape errors belong to Validate", err)
	}
	if err := got.Validate(); !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidBatch", err)
	}
}

const fixturesDir = "../../fixtures"

var validFixtures = []string{
	"segments.json",
	"placeholders.json",
	"markup.json",
	"names.json",
	"numbers.json",
	"empty.json",
	"mixed-language.json",
}

func loadFixture(t *testing.T, name string) batch.TranslationBatch {
	t.Helper()

	file, err := os.Open(filepath.Join(fixturesDir, name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer file.Close()

	got, err := batch.Load(file)
	if err != nil {
		t.Fatalf("Load(%s) = %v, want nil", name, err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate(%s) = %v, want nil", name, err)
	}
	return got
}

func TestEveryValidFixtureLoadsAndValidates(t *testing.T) {
	for _, name := range validFixtures {
		t.Run(name, func(t *testing.T) {
			loadFixture(t, name)
		})
	}
}

func TestNoSegmentsFixtureFailsValidation(t *testing.T) {
	file, err := os.Open(filepath.Join(fixturesDir, "no-segments.json"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	got, err := batch.Load(file)
	if err != nil {
		t.Fatalf("Load(no-segments.json) = %v, want nil", err)
	}
	if err := got.Validate(); !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate(no-segments.json) = %v, want an error wrapping ErrInvalidBatch", err)
	}
}

func TestFixturesCoverTheRequiredCases(t *testing.T) {
	joined := &strings.Builder{}
	whitespaceOnly := false
	nonEnglish := false

	for _, name := range validFixtures {
		got := loadFixture(t, name)
		for _, segment := range got.Segments {
			joined.WriteString(segment.Text)
			joined.WriteString("\n")
			if strings.TrimSpace(segment.Text) == "" {
				whitespaceOnly = true
			}
		}
		if got.TargetLanguage == "fr" {
			nonEnglish = true
		}
	}

	text := joined.String()
	for _, want := range []string{"%s", "%d", "%1$s", "%%", "{count}", "{0, plural", "<b>", "</b>", "&amp;", "&#233;", "1,234.56"} {
		if !strings.Contains(text, want) {
			t.Errorf("the fixtures do not cover %q", want)
		}
	}
	if !whitespaceOnly {
		t.Error("the fixtures do not cover a whitespace-only segment")
	}
	if !nonEnglish {
		t.Error("the fixtures do not cover a mixed-language batch")
	}
}
