package translation

import (
	"fmt"
	"strings"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/tokens"
)

// Build checks a model answer against the batch it answers and returns the batch
// that may be published as a draft.
//
// It enforces, in this order: every input segment comes back exactly once with
// its own identifier and in the input order; an empty or whitespace-only segment
// stays empty; and each segment preserves its placeholders, markup, numbers, and
// glossary names as multisets. keep is the glossary passed to tokens.Extract.
//
// The returned batch always carries draft status on every segment and the
// human-review warnings; the caller decides whether to present it.
func Build(in batch.TranslationBatch, candidate Candidate, keep []string) (TranslatedBatch, error) {
	problems := checkShape(in, candidate)
	if len(problems) > 0 {
		return TranslatedBatch{}, &ValidationError{Problems: problems}
	}

	produced := make(map[string]string, len(candidate.Segments))
	for _, segment := range candidate.Segments {
		produced[segment.ID] = segment.Text
	}

	for _, segment := range in.Segments {
		text := produced[segment.ID]
		if strings.TrimSpace(segment.Text) == "" {
			if strings.TrimSpace(text) != "" {
				problems = append(problems, Problem{
					Code:      "empty_segment_not_empty",
					SegmentID: segment.ID,
					Detail:    fmt.Sprintf("expected an empty translation, got %q", text),
				})
			}
			continue
		}
		problems = append(problems, checkTokens(segment, text, keep)...)
	}

	if len(problems) > 0 {
		return TranslatedBatch{}, &ValidationError{Problems: problems}
	}

	out := TranslatedBatch{
		SourceLanguage: in.SourceLanguage,
		TargetLanguage: in.TargetLanguage,
		Segments:       make([]Segment, 0, len(in.Segments)),
		Warnings:       []string{draftWarning},
	}
	if _, gated := gatedAudiences[strings.ToLower(strings.TrimSpace(in.Audience))]; gated {
		out.Warnings = append(out.Warnings, releaseWarning)
	}
	for _, segment := range in.Segments {
		out.Segments = append(out.Segments, Segment{
			ID:     segment.ID,
			Text:   produced[segment.ID],
			Status: DraftStatus,
		})
	}

	return out, nil
}

// checkShape enforces rules R1 and R2: the candidate answers every input segment
// exactly once and in the input order.
//
// The order check runs only when the identifier sets already match, so one
// missing segment is reported as missing instead of cascading into a reorder
// report for every remaining position.
func checkShape(in batch.TranslationBatch, candidate Candidate) []Problem {
	var problems []Problem

	asked := make(map[string]struct{}, len(in.Segments))
	for _, segment := range in.Segments {
		asked[segment.ID] = struct{}{}
	}

	answered := make(map[string]struct{}, len(candidate.Segments))
	for _, segment := range candidate.Segments {
		id := strings.TrimSpace(segment.ID)
		if _, duplicated := answered[id]; duplicated {
			problems = append(problems, Problem{
				Code:      "segment_duplicate",
				SegmentID: id,
				Detail:    "the candidate returned this segment more than once",
			})
			continue
		}
		answered[id] = struct{}{}
		if _, exists := asked[id]; !exists {
			problems = append(problems, Problem{
				Code:      "segment_unknown_id",
				SegmentID: id,
				Detail:    "the candidate returned a segment the batch never asked for",
			})
		}
	}

	for _, segment := range in.Segments {
		if _, ok := answered[segment.ID]; !ok {
			problems = append(problems, Problem{
				Code:      "segment_missing",
				SegmentID: segment.ID,
				Detail:    "the candidate did not return this segment",
			})
		}
	}
	if len(problems) > 0 {
		return problems
	}

	for index, segment := range in.Segments {
		if candidate.Segments[index].ID != segment.ID {
			problems = append(problems, Problem{
				Code:      "segment_out_of_order",
				SegmentID: segment.ID,
				Detail:    fmt.Sprintf("the candidate returned %q at position %d", candidate.Segments[index].ID, index),
			})
		}
	}

	return problems
}

// checkTokens enforces rules R4 to R6 for one segment by comparing protected
// spans as multisets in both directions.
func checkTokens(segment batch.Segment, produced string, keep []string) []Problem {
	want := tokens.Extract(segment.Text, keep)
	got := tokens.Extract(produced, keep)

	var problems []Problem
	for _, line := range want.Diff(got) {
		kind, direction, detail := splitDiffLine(line)
		problems = append(problems, Problem{
			Code:      kind + "_" + direction,
			SegmentID: segment.ID,
			Detail:    detail,
		})
	}

	return problems
}

// splitDiffLine reads one tokens.Diff line, which has the form
// "<kind> <missing|extra>: <value>". The value keeps any later ": " it contains,
// so a tag carrying a URL survives the split intact.
func splitDiffLine(line string) (kind, direction, detail string) {
	label, value, _ := strings.Cut(line, ": ")
	kind, direction, _ = strings.Cut(label, " ")

	return kind, direction, value
}
