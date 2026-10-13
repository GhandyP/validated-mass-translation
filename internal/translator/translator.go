// Package translator is the model seam: it turns a translation batch into the
// candidate answer that the translation rules then validate.
//
// Only this package knows about Google ADK. Everything downstream compares plain
// Go values, which is why the whole rule set can be exercised offline with Fake
// and no API key.
package translator

import (
	"context"
	"errors"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
)

// ErrModelUnavailable is wrapped by every failure that means "the model could not
// answer": a missing key, a model that cannot be constructed, a run failure, and
// a structured output that cannot be decoded.
var ErrModelUnavailable = errors.New("model unavailable")

// Translator answers one translation batch.
type Translator interface {
	// Translate returns one candidate segment per input segment. Its answer is
	// not trusted: translation.Build decides whether it may be published.
	Translate(ctx context.Context, in batch.TranslationBatch, keep []string) (translation.Candidate, error)
}

// Options configures the Gemini translator.
type Options struct {
	// Model overrides the model name. Empty means the ADK_MODEL environment
	// variable, and then the stable default for this repository.
	Model string
}

// Fake is a deterministic translator for tests and for the offline golden files.
//
// It answers every input segment in input order, mapping known ids through ByID
// and returning an empty text for the rest, which is exactly what an input
// segment with nothing to translate looks like.
type Fake struct {
	ByID map[string]string
	Fail error
}

var _ Translator = Fake{}

// Translate answers from the mapping, or returns Fail.
func (f Fake) Translate(ctx context.Context, in batch.TranslationBatch, keep []string) (translation.Candidate, error) {
	if f.Fail != nil {
		return translation.Candidate{}, f.Fail
	}

	segments := make([]translation.CandidateSegment, 0, len(in.Segments))
	for _, segment := range in.Segments {
		segments = append(segments, translation.CandidateSegment{ID: segment.ID, Text: f.ByID[segment.ID]})
	}

	return translation.Candidate{Segments: segments}, nil
}
