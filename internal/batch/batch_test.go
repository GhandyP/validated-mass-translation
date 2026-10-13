package batch_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
)

func validBatch() batch.TranslationBatch {
	return batch.TranslationBatch{
		SourceLanguage: "en",
		TargetLanguage: "es",
		Segments: []batch.Segment{
			{ID: "cart.title", Text: "Add %s to the cart"},
			{ID: "layout.spacer", Text: "   "},
		},
	}
}

func TestValidateAcceptsAWellFormedBatch(t *testing.T) {
	if err := validBatch().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateAcceptsAnOptionalAudience(t *testing.T) {
	b := validBatch()
	b.Audience = "customer-facing"
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() with an audience = %v, want nil", err)
	}
}

func TestValidateRejectsABatchWithoutSegments(t *testing.T) {
	b := validBatch()
	b.Segments = nil

	err := b.Validate()
	if !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidBatch", err)
	}
	if !strings.Contains(err.Error(), "segment") {
		t.Fatalf("Validate() error = %q, want it to mention segments", err.Error())
	}
}

func TestValidateRejectsABlankSegmentID(t *testing.T) {
	b := validBatch()
	b.Segments[1].ID = "  "

	err := b.Validate()
	if !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidBatch", err)
	}
	if !strings.Contains(err.Error(), "blank") {
		t.Fatalf("Validate() error = %q, want it to mention the blank id", err.Error())
	}
}

func TestValidateRejectsADuplicateSegmentID(t *testing.T) {
	b := validBatch()
	b.Segments[1].ID = "cart.title"

	err := b.Validate()
	if !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidBatch", err)
	}
	if !strings.Contains(err.Error(), "cart.title") {
		t.Fatalf("Validate() error = %q, want it to name the duplicate id", err.Error())
	}
}

func TestValidateRejectsABlankSourceLanguage(t *testing.T) {
	b := validBatch()
	b.SourceLanguage = " \t "

	err := b.Validate()
	if !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidBatch", err)
	}
	if !strings.Contains(err.Error(), "sourceLanguage") {
		t.Fatalf("Validate() error = %q, want it to name sourceLanguage", err.Error())
	}
}

func TestValidateRejectsABlankTargetLanguage(t *testing.T) {
	b := validBatch()
	b.TargetLanguage = ""

	err := b.Validate()
	if !errors.Is(err, batch.ErrInvalidBatch) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidBatch", err)
	}
	if !strings.Contains(err.Error(), "targetLanguage") {
		t.Fatalf("Validate() error = %q, want it to name targetLanguage", err.Error())
	}
}

func TestValidateDoesNotReorderSegments(t *testing.T) {
	b := validBatch()
	b.Segments = []batch.Segment{{ID: "b"}, {ID: "a"}}

	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if b.Segments[0].ID != "b" || b.Segments[1].ID != "a" {
		t.Fatalf("Validate() reordered segments: got %q, %q", b.Segments[0].ID, b.Segments[1].ID)
	}
}
