// Package batch models the translation input contract: one batch of text
// segments together with the language tags the caller asks for.
//
// The package is pure and offline: it reads JSON, checks the batch shape, and
// never reorders, trims, or rewrites segment text, because the identifiers and
// the order are part of what the caller can observe downstream.
package batch

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidBatch is wrapped by every input error this package returns, so a
// caller can test for it with errors.Is instead of matching error strings.
var ErrInvalidBatch = errors.New("invalid translation batch")

// Segment is one translatable unit of text with a stable identifier.
type Segment struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// TranslationBatch is one translation request: the language the segments are
// written in, the language the caller wants them translated into, the optional
// audience that gates publication, and the ordered segments themselves.
type TranslationBatch struct {
	SourceLanguage string    `json:"sourceLanguage"`
	TargetLanguage string    `json:"targetLanguage"`
	Audience       string    `json:"audience,omitempty"`
	Segments       []Segment `json:"segments"`
}

// Validate checks the batch shape: non-blank language tags, at least one
// segment, and identifiers that are non-blank and unique.
//
// Audience is optional metadata and is never validated, and no language is
// detected or compared: a batch may legitimately mix languages, and a language
// check would need a detector this package deliberately does not depend on.
func (b TranslationBatch) Validate() error {
	if strings.TrimSpace(b.SourceLanguage) == "" {
		return fmt.Errorf("%w: sourceLanguage is blank", ErrInvalidBatch)
	}
	if strings.TrimSpace(b.TargetLanguage) == "" {
		return fmt.Errorf("%w: targetLanguage is blank", ErrInvalidBatch)
	}
	if len(b.Segments) == 0 {
		return fmt.Errorf("%w: a batch needs at least one segment", ErrInvalidBatch)
	}

	seen := make(map[string]struct{}, len(b.Segments))
	for index, segment := range b.Segments {
		id := strings.TrimSpace(segment.ID)
		if id == "" {
			return fmt.Errorf("%w: segment %d has a blank id", ErrInvalidBatch, index)
		}
		if _, duplicated := seen[id]; duplicated {
			return fmt.Errorf("%w: duplicate segment id %q", ErrInvalidBatch, id)
		}
		seen[id] = struct{}{}
	}

	return nil
}
