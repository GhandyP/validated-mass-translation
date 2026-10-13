package locale_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/locale"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
)

func write(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestLoadReadsAFlatLocaleFile(t *testing.T) {
	path := write(t, "es.json", "{\n  \"cart.title\": \"Agregá %s al carrito\",\n  \"cart.note\": \"\"\n}\n")

	got, err := locale.Load(path)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if len(got) != 2 || got["cart.title"] != "Agregá %s al carrito" || got["cart.note"] != "" {
		t.Fatalf("Load() = %v, want the two entries of the file", got)
	}
}

func TestLoadRejectsMalformedLocaleFiles(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "empty file", content: ""},
		{name: "not json", content: "cart.title=Add %s\n"},
		{name: "array instead of object", content: "[\"cart.title\"]\n"},
		{name: "numeric value", content: "{\"cart.title\": 7}\n"},
		{name: "nested value", content: "{\"cart.title\": {\"text\": \"x\"}}\n"},
		{name: "trailing data", content: "{\"cart.title\": \"x\"} extra\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := write(t, "es.json", test.content)

			if _, err := locale.Load(path); err == nil {
				t.Fatalf("Load(%q) = nil, want an error", test.content)
			}
		})
	}
}

func TestLoadReportsAMissingFile(t *testing.T) {
	_, err := locale.Load(filepath.Join(t.TempDir(), "absent.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() = %v, want an error wrapping os.ErrNotExist", err)
	}
}

func TestBundleUsesTheSegmentIDs(t *testing.T) {
	batch := translation.TranslatedBatch{
		Segments: []translation.Segment{
			{ID: "cart.title", Text: "Agregá %s al carrito", Status: translation.DraftStatus},
			{ID: "cart.note", Text: "", Status: translation.DraftStatus},
		},
	}

	got := locale.Bundle(batch)
	if len(got) != 2 || got["cart.title"] != "Agregá %s al carrito" || got["cart.note"] != "" {
		t.Fatalf("Bundle() = %v, want one entry per segment id", got)
	}
}

func TestDiffIsSortedAndReportsEveryKindOfChange(t *testing.T) {
	existing := map[string]string{
		"cart.title":  "Añadir %s al carrito",
		"cart.legacy": "Texto viejo",
		"cart.total":  "Total: 1,234.56 USD",
	}
	candidate := map[string]string{
		"cart.title":   "Agregá %s al carrito",
		"cart.note":    "",
		"cart.total":   "Total: 1,234.56 USD",
		"promo.banner": "Ahorrá <b>10%</b> en tu pedido",
	}

	got := locale.Diff(existing, candidate)

	want := []string{
		"- cart.legacy: \"Texto viejo\"",
		"+ cart.note: \"\"",
		"~ cart.title: \"Añadir %s al carrito\" -> \"Agregá %s al carrito\"",
		"+ promo.banner: \"Ahorrá <b>10%</b> en tu pedido\"",
	}

	lines := bytes.Split([]byte(got), []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) != len(want) {
		t.Fatalf("Diff() =\n%s\nwant %d lines", got, len(want))
	}
	for index, line := range lines {
		if string(line) != want[index] {
			t.Fatalf("Diff() line %d = %q, want %q", index, line, want[index])
		}
	}
}

func TestDiffIsEmptyWhenNothingChanged(t *testing.T) {
	existing := map[string]string{"cart.title": "Agregá %s al carrito"}

	if got := locale.Diff(existing, existing); got != "" {
		t.Fatalf("Diff() = %q, want an empty string", got)
	}
}

func TestDiffReportsAFirstPublishAsAdditions(t *testing.T) {
	got := locale.Diff(nil, map[string]string{"cart.title": "Agregá %s al carrito"})

	if got != "+ cart.title: \"Agregá %s al carrito\"\n" {
		t.Fatalf("Diff() = %q, want a single addition line", got)
	}
}
