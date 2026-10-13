package translation_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
)

const (
	draftWarning   = "machine translation: every segment is a draft"
	releaseWarning = "a human reviewer must approve legal, medical, or customer-facing releases before publication"
)

func input(segments ...batch.Segment) batch.TranslationBatch {
	return batch.TranslationBatch{SourceLanguage: "en", TargetLanguage: "es", Segments: segments}
}

func candidate(segments ...translation.CandidateSegment) translation.Candidate {
	return translation.Candidate{Segments: segments}
}

func problemsOf(t *testing.T, err error) []translation.Problem {
	t.Helper()

	var invalid *translation.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("Build() error = %v, want a *translation.ValidationError", err)
	}
	if len(invalid.Problems) == 0 {
		t.Fatal("Build() returned a ValidationError without problems")
	}
	return invalid.Problems
}

func codesOf(problems []translation.Problem) []string {
	codes := make([]string, 0, len(problems))
	for _, problem := range problems {
		codes = append(codes, problem.Code)
	}
	return codes
}

func equalStrings(t *testing.T, label string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}

func TestBuildAcceptsAGroundedCandidate(t *testing.T) {
	in := input(
		batch.Segment{ID: "cart.title", Text: "Add %s to the cart"},
		batch.Segment{ID: "promo.banner", Text: "Save <b>10%</b> on your order"},
		batch.Segment{ID: "cart.total", Text: "Total: 1,234.56 USD"},
	)
	got, err := translation.Build(in, candidate(
		translation.CandidateSegment{ID: "cart.title", Text: "Agregá %s al carrito"},
		translation.CandidateSegment{ID: "promo.banner", Text: "Ahorrá <b>10%</b> en tu pedido"},
		translation.CandidateSegment{ID: "cart.total", Text: "Total: 1,234.56 USD"},
	), nil)

	if err != nil {
		t.Fatalf("Build() = %v, want nil", err)
	}
	if len(got.Segments) != 3 {
		t.Fatalf("Build() produced %d segments, want 3", len(got.Segments))
	}
	for index, segment := range got.Segments {
		if segment.Status != translation.DraftStatus {
			t.Errorf("segment %d status = %q, want %q", index, segment.Status, translation.DraftStatus)
		}
		if segment.ID != in.Segments[index].ID {
			t.Errorf("segment %d id = %q, want %q", index, segment.ID, in.Segments[index].ID)
		}
	}
	equalStrings(t, "warnings", got.Warnings, []string{draftWarning})
}

func TestBuildEchoesTheRequestedLanguages(t *testing.T) {
	got, err := translation.Build(
		input(batch.Segment{ID: "a", Text: "Ready"}),
		candidate(translation.CandidateSegment{ID: "a", Text: "Listo"}),
		nil,
	)
	if err != nil {
		t.Fatalf("Build() = %v, want nil", err)
	}
	if got.SourceLanguage != "en" || got.TargetLanguage != "es" {
		t.Fatalf("Build() languages = %q -> %q, want en -> es", got.SourceLanguage, got.TargetLanguage)
	}
}

func TestBuildAddsTheReleaseWarningOnlyForGatedAudiences(t *testing.T) {
	tests := []struct {
		audience string
		want     bool
	}{
		{audience: "", want: false},
		{audience: "internal", want: false},
		{audience: "engineering", want: false},
		{audience: "customer-facing", want: true},
		{audience: "legal", want: true},
		{audience: "medical", want: true},
		{audience: " Medical ", want: true},
		{audience: "LEGAL", want: true},
	}

	for _, test := range tests {
		t.Run("audience="+test.audience, func(t *testing.T) {
			in := input(batch.Segment{ID: "a", Text: "Ready"})
			in.Audience = test.audience

			got, err := translation.Build(in, candidate(translation.CandidateSegment{ID: "a", Text: "Listo"}), nil)
			if err != nil {
				t.Fatalf("Build() = %v, want nil", err)
			}

			want := []string{draftWarning}
			if test.want {
				want = append(want, releaseWarning)
			}
			equalStrings(t, "warnings", got.Warnings, want)
		})
	}
}

func TestBuildReportsAMissingSegment(t *testing.T) {
	in := input(
		batch.Segment{ID: "a", Text: "Ready"},
		batch.Segment{ID: "b", Text: "Also ready"},
	)

	_, err := translation.Build(in, candidate(translation.CandidateSegment{ID: "a", Text: "Listo"}), nil)
	got := problemsOf(t, err)

	equalStrings(t, "codes", codesOf(got), []string{"segment_missing"})
	if got[0].SegmentID != "b" {
		t.Fatalf("problem segment = %q, want b", got[0].SegmentID)
	}
}

func TestBuildReportsAnUnknownSegment(t *testing.T) {
	in := input(batch.Segment{ID: "a", Text: "Ready"})

	_, err := translation.Build(in, candidate(
		translation.CandidateSegment{ID: "a", Text: "Listo"},
		translation.CandidateSegment{ID: "ghost", Text: "Inventado"},
	), nil)
	got := problemsOf(t, err)

	equalStrings(t, "codes", codesOf(got), []string{"segment_unknown_id"})
	if got[0].SegmentID != "ghost" {
		t.Fatalf("problem segment = %q, want ghost", got[0].SegmentID)
	}
}

func TestBuildReportsADuplicateSegment(t *testing.T) {
	in := input(
		batch.Segment{ID: "a", Text: "Ready"},
		batch.Segment{ID: "b", Text: "Also ready"},
	)

	_, err := translation.Build(in, candidate(
		translation.CandidateSegment{ID: "a", Text: "Listo"},
		translation.CandidateSegment{ID: "a", Text: "Listo otra vez"},
	), nil)
	got := problemsOf(t, err)

	equalStrings(t, "codes", codesOf(got), []string{"segment_duplicate", "segment_missing"})
	if got[0].SegmentID != "a" || got[1].SegmentID != "b" {
		t.Fatalf("problem segments = %q, %q, want a, b", got[0].SegmentID, got[1].SegmentID)
	}
}

func TestBuildReportsAReorder(t *testing.T) {
	in := input(
		batch.Segment{ID: "a", Text: "Ready"},
		batch.Segment{ID: "b", Text: "Also ready"},
	)

	_, err := translation.Build(in, candidate(
		translation.CandidateSegment{ID: "b", Text: "También listo"},
		translation.CandidateSegment{ID: "a", Text: "Listo"},
	), nil)
	got := problemsOf(t, err)

	equalStrings(t, "codes", codesOf(got), []string{"segment_out_of_order", "segment_out_of_order"})
	equalStrings(t, "segments", []string{got[0].SegmentID, got[1].SegmentID}, []string{"a", "b"})
	if !strings.Contains(got[0].Detail, "b") {
		t.Fatalf("problem detail = %q, want it to name the segment that arrived instead", got[0].Detail)
	}
}

func TestBuildKeepsAnEmptySegmentEmpty(t *testing.T) {
	in := input(batch.Segment{ID: "spacer", Text: "   \n  "})

	got, err := translation.Build(in, candidate(translation.CandidateSegment{ID: "spacer", Text: "  "}), nil)
	if err != nil {
		t.Fatalf("Build() = %v, want nil for an empty translation of an empty segment", err)
	}
	if got.Segments[0].Text != "  " {
		t.Fatalf("segment text = %q, want the candidate text unchanged", got.Segments[0].Text)
	}
}

func TestBuildRejectsANonEmptyTranslationOfAnEmptySegment(t *testing.T) {
	in := input(batch.Segment{ID: "spacer", Text: " \t "})

	_, err := translation.Build(in, candidate(translation.CandidateSegment{ID: "spacer", Text: "algo"}), nil)
	got := problemsOf(t, err)

	equalStrings(t, "codes", codesOf(got), []string{"empty_segment_not_empty"})
	if got[0].SegmentID != "spacer" {
		t.Fatalf("problem segment = %q, want spacer", got[0].SegmentID)
	}
}

func TestBuildReportsTokenDrift(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		target       string
		keep         []string
		wantCodes    []string
		wantSegments []string
		wantDetail   string
	}{
		{
			name:         "placeholder missing",
			source:       "Add %s to the cart",
			target:       "Añadir al carrito",
			wantCodes:    []string{"placeholder_missing"},
			wantSegments: []string{"s"},
			wantDetail:   "%s",
		},
		{
			name:         "placeholder extra",
			source:       "Add to the cart",
			target:       "Añadir %s al carrito",
			wantCodes:    []string{"placeholder_extra"},
			wantSegments: []string{"s"},
			wantDetail:   "%s",
		},
		{
			name:         "icu placeholder missing",
			source:       "Hello {name}",
			target:       "Hola",
			wantCodes:    []string{"placeholder_missing"},
			wantSegments: []string{"s"},
			wantDetail:   "{name}",
		},
		{
			name:         "markup missing",
			source:       "Bread &amp; butter",
			target:       "Pan y mantequilla",
			wantCodes:    []string{"markup_missing"},
			wantSegments: []string{"s"},
			wantDetail:   "&amp;",
		},
		{
			name:         "markup extra",
			source:       "Pan y mantequilla",
			target:       "Bread &amp; butter",
			wantCodes:    []string{"markup_extra"},
			wantSegments: []string{"s"},
			wantDetail:   "&amp;",
		},
		{
			name:         "tag missing",
			source:       "This is <b>important</b>",
			target:       "Esto es importante",
			wantCodes:    []string{"markup_missing", "markup_missing"},
			wantSegments: []string{"s", "s"},
			wantDetail:   "</b>",
		},
		{
			name:         "number missing",
			source:       "42 pending requests",
			target:       "Solicitudes pendientes",
			wantCodes:    []string{"number_missing"},
			wantSegments: []string{"s"},
			wantDetail:   "42",
		},
		{
			name:         "number extra",
			source:       "Pending requests",
			target:       "42 solicitudes pendientes",
			wantCodes:    []string{"number_extra"},
			wantSegments: []string{"s"},
			wantDetail:   "42",
		},
		{
			name:         "glossary name missing",
			source:       "Acme ships this week",
			target:       "La empresa envía esta semana",
			keep:         []string{"Acme"},
			wantCodes:    []string{"name_missing"},
			wantSegments: []string{"s"},
			wantDetail:   "Acme",
		},
		{
			name:         "glossary name extra",
			source:       "La empresa envía esta semana",
			target:       "Acme envía esta semana",
			keep:         []string{"Acme"},
			wantCodes:    []string{"name_extra"},
			wantSegments: []string{"s"},
			wantDetail:   "Acme",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := input(batch.Segment{ID: "s", Text: test.source})

			_, err := translation.Build(in, candidate(translation.CandidateSegment{ID: "s", Text: test.target}), test.keep)
			if err == nil {
				t.Fatalf("Build() = nil, want %v", test.wantCodes)
			}
			got := problemsOf(t, err)

			equalStrings(t, "codes", codesOf(got), test.wantCodes)
			segments := make([]string, 0, len(got))
			for _, problem := range got {
				segments = append(segments, problem.SegmentID)
			}
			equalStrings(t, "segments", segments, test.wantSegments)
			if !strings.Contains(got[0].Detail, test.wantDetail) {
				t.Fatalf("problem detail = %q, want it to contain %q", got[0].Detail, test.wantDetail)
			}
		})
	}
}

func TestBuildAcceptsARewordingThatKeepsEveryToken(t *testing.T) {
	tests := []struct {
		name   string
		source string
		target string
		keep   []string
	}{
		{name: "printf verb", source: "Add %s to the cart", target: "Agregá %s al carrito"},
		{name: "icu expression", source: "Hello {name}, welcome", target: "Hola {name}, bienvenido"},
		{name: "nested plural", source: "You have {0, plural, one{# item} other{# items}}", target: "Tenés {0, plural, one{# ítem} other{# ítems}}"},
		{name: "markup", source: "Save <b>10%</b> now", target: "Ahorrá <b>10%</b> ya"},
		{name: "entity", source: "Bread &amp; butter", target: "Pan &amp; manteca"},
		{name: "number", source: "Total 1,234.56 USD", target: "Total 1,234.56 USD"},
		{name: "glossary", source: "Acme ships this week", target: "Acme envía esta semana", keep: []string{"Acme"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := input(batch.Segment{ID: "s", Text: test.source})

			if _, err := translation.Build(in, candidate(translation.CandidateSegment{ID: "s", Text: test.target}), test.keep); err != nil {
				t.Fatalf("Build() = %v, want nil", err)
			}
		})
	}
}

func TestBuildReportsEveryProblemWithItsSegment(t *testing.T) {
	in := input(
		batch.Segment{ID: "a", Text: "Ready"},
		batch.Segment{ID: "b", Text: "Total %s"},
	)

	_, err := translation.Build(in, candidate(
		translation.CandidateSegment{ID: "a", Text: "Listo"},
		translation.CandidateSegment{ID: "b", Text: "Total"},
	), nil)
	got := problemsOf(t, err)

	equalStrings(t, "codes", codesOf(got), []string{"placeholder_missing"})
	if got[0].SegmentID != "b" {
		t.Fatalf("problem segment = %q, want b", got[0].SegmentID)
	}
	if !strings.Contains(err.Error(), "b") || !strings.Contains(err.Error(), "placeholder_missing") {
		t.Fatalf("error = %q, want it to name the segment and the code", err.Error())
	}
}

func TestBuildProducesTheFrozenWireFormat(t *testing.T) {
	in := input(
		batch.Segment{ID: "a", Text: "Hello %s"},
		batch.Segment{ID: "b", Text: "<b>Save</b> 1,234.56"},
	)

	got, err := translation.Build(in, candidate(
		translation.CandidateSegment{ID: "a", Text: "Hola %s"},
		translation.CandidateSegment{ID: "b", Text: "<b>Guarda</b> 1,234.56"},
	), nil)
	if err != nil {
		t.Fatalf("Build() = %v, want nil", err)
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(got); err != nil {
		t.Fatalf("encode: %v", err)
	}

	want := "{\"sourceLanguage\":\"en\",\"targetLanguage\":\"es\",\"segments\":[{\"id\":\"a\",\"text\":\"Hola %s\",\"status\":\"draft\"},{\"id\":\"b\",\"text\":\"<b>Guarda</b> 1,234.56\",\"status\":\"draft\"}],\"warnings\":[\"machine translation: every segment is a draft\"]}\n"
	if buffer.String() != want {
		t.Fatalf("wire format =\n%s\nwant\n%s", buffer.String(), want)
	}
}
