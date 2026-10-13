//go:build integration

package translator_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
	"github.com/GhandyP/validated-mass-translation/internal/translator"
)

// TestGeminiTranslatesABatch exercises the live Gemini path. It is OptIn: the
// integration build tag plus a GOOGLE_API_KEY, and it skips instead of failing
// when either is missing, so the offline suite never claims a live result.
func TestGeminiTranslatesABatch(t *testing.T) {
	if strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")) == "" {
		t.Skip("GOOGLE_API_KEY is not set; the live Gemini path is opt-in")
	}

	in := batch.TranslationBatch{
		SourceLanguage: "en",
		TargetLanguage: "es",
		Segments: []batch.Segment{
			{ID: "cart.title", Text: "Add %s to the cart"},
			{ID: "promo.banner", Text: "Save <b>10%</b> on your order"},
			{ID: "cart.total", Text: "Total: 1,234.56 USD"},
			{ID: "layout.spacer", Text: "   "},
		},
	}

	live, err := translator.NewGemini(context.Background(), translator.Options{})
	if err != nil {
		t.Fatalf("NewGemini() = %v", err)
	}

	candidate, err := live.Translate(context.Background(), in, []string{"Acme"})
	if err != nil {
		t.Fatalf("Translate() = %v", err)
	}
	if _, err := translation.Build(in, candidate, []string{"Acme"}); err != nil {
		t.Fatalf("Build() = %v, want a candidate that preserves every protected span", err)
	}
}
