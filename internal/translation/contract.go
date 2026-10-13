// Package translation models the output side of a batch translation: the
// contract the caller receives, and the rules that decide whether a model answer
// may be published as a draft.
//
// The model returns identifiers and text, and nothing else. Draft status, both
// language tags, and the warnings are set here, so a model cannot claim a review
// that never happened or a status it does not own.
package translation

import "strings"

// DraftStatus marks every emitted segment as machine output that a human has not
// reviewed yet.
const DraftStatus = "draft"

const (
	// draftWarning is always attached to an emitted batch.
	draftWarning = "machine translation: every segment is a draft"
	// releaseWarning is attached when the batch declares a gated audience.
	releaseWarning = "a human reviewer must approve legal, medical, or customer-facing releases before publication"
)

// gatedAudiences lists the audiences whose releases need a human reviewer before
// publication, matched case-insensitively after trimming.
var gatedAudiences = map[string]struct{}{
	"legal":           {},
	"medical":         {},
	"customer-facing": {},
}

// CandidateSegment is one segment as the model returned it: no status, no
// language, no warning.
type CandidateSegment struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Candidate is the model answer as a whole.
type Candidate struct {
	Segments []CandidateSegment `json:"segments"`
}

// Segment is one segment of an emitted batch.
type Segment struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// TranslatedBatch is what a caller may publish as a draft: the requested
// languages, one segment per input segment in input order, and the warnings that
// apply to this batch.
type TranslatedBatch struct {
	SourceLanguage string    `json:"sourceLanguage"`
	TargetLanguage string    `json:"targetLanguage"`
	Segments       []Segment `json:"segments"`
	Warnings       []string  `json:"warnings"`
}

// Problem is one rule violation, identified by a stable code so callers and
// tests can match on it instead of on prose.
type Problem struct {
	Code      string `json:"code"`
	SegmentID string `json:"segmentID"`
	Detail    string `json:"detail"`
}

// ValidationError carries every problem found in one candidate, in a
// deterministic order, so a caller can report all of them at once.
type ValidationError struct {
	Problems []Problem
}

// Error renders the problems as "segment: code: detail" entries.
func (e *ValidationError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return "translation validation failed"
	}

	entries := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		if problem.SegmentID == "" {
			entries = append(entries, problem.Code+": "+problem.Detail)
			continue
		}
		entries = append(entries, problem.SegmentID+": "+problem.Code+": "+problem.Detail)
	}

	return "translation validation failed: " + strings.Join(entries, "; ")
}
