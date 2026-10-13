package tokens_test

import (
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/tokens"
)

func TestExtractFindsPrintfVerbs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "string verb", text: "Hello %s", want: []string{"%s"}},
		{name: "decimal verb", text: "You have %d messages", want: []string{"%d"}},
		{name: "value verb", text: "Got %v back", want: []string{"%v"}},
		{name: "positional verb", text: "%1$s sent %1$s a copy", want: []string{"%1$s", "%1$s"}},
		{name: "several verbs keep source order", text: "%d then %s", want: []string{"%d", "%s"}},
		{name: "escaped percent is not a verb", text: "100%% done", want: nil},
		{name: "lone percent is not a verb", text: "Save 10%</b> now", want: nil},
		{name: "trailing percent is not a verb", text: "almost %", want: nil},
		{name: "any single letter verb is collected", text: "keep %q as written", want: []string{"%q"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tokens.Extract(test.text, nil)
			assertEqual(t, "placeholders", got.Placeholders, test.want)
		})
	}
}

func TestExtractFindsICUExpressions(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "named argument", text: "Hello {name}", want: []string{"{name}"}},
		{name: "numbered argument", text: "Hello {0}", want: []string{"{0}"}},
		{name: "plural expression", text: "You have {0, plural, one{# item} other{# items}}", want: []string{"{0, plural, one{# item} other{# items}}"}},
		{name: "select expression with nesting", text: "{gender, select, male{He} female{She} other{They}} replied", want: []string{"{gender, select, male{He} female{She} other{They}}"}},
		{name: "several expressions keep source order", text: "{a} and {b}", want: []string{"{a}", "{b}"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tokens.Extract(test.text, nil)
			assertEqual(t, "placeholders", got.Placeholders, test.want)
		})
	}
}

func TestExtractNeverPanicsOnUnbalancedBraces(t *testing.T) {
	// A dangling brace is reported as an ICU expression running to the end of
	// the text, so a truncated translation cannot silently hide a placeholder.
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "open brace runs to the end", text: "Hello {name", want: []string{"{name"}},
		{name: "open brace alone", text: "tail {", want: []string{"{"}},
		{name: "stray close brace is not an expression", text: "closing } alone", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tokens.Extract(test.text, nil)
			assertEqual(t, "placeholders", got.Placeholders, test.want)
		})
	}
}

func TestExtractFindsMarkup(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "opening and closing tags", text: "This is <b>important</b>", want: []string{"<b>", "</b>"}},
		{name: "tag with attributes", text: "Read <a href=\"https://example.com/docs\">the docs</a>", want: []string{"<a href=\"https://example.com/docs\">", "</a>"}},
		{name: "entities", text: "Bread &amp; butter, caf&#233;", want: []string{"&amp;", "&#233;"}},
		{name: "comparison is not markup", text: "1 < 2 and 3 > 2", want: nil},
		{name: "unterminated tag is ignored", text: "open <b without close", want: nil},
		{name: "unterminated entity is ignored", text: "bread &amp butter", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tokens.Extract(test.text, nil)
			assertEqual(t, "markup", got.Markup, test.want)
		})
	}
}

func TestExtractFindsNumbers(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "integer", text: "42 pending", want: []string{"42"}},
		{name: "decimal", text: "3.14 seconds", want: []string{"3.14"}},
		{name: "thousands separator", text: "1,234.56 USD", want: []string{"1,234.56"}},
		{name: "negative", text: "-7 points", want: []string{"-7"}},
		{name: "several keep source order", text: "from 1 to 2.5", want: []string{"1", "2.5"}},
		{name: "digits inside an entity are not numbers", text: "caf&#233; costs 3", want: []string{"3"}},
		{name: "digits inside a tag are not numbers", text: "<a href=\"plan2\">go</a> 5", want: []string{"5"}},
		{name: "digits inside an ICU expression are not numbers", text: "You have {0, plural, one{# item} other{# items}}", want: nil},
		{name: "positional printf digits are not numbers", text: "%1$s sent a copy", want: nil},
		{name: "no digits", text: "nothing here", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tokens.Extract(test.text, nil)
			assertEqual(t, "numbers", got.Numbers, test.want)
		})
	}
}

func TestExtractFindsGlossaryNames(t *testing.T) {
	keep := []string{"Acme", "Northwind"}

	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "each term found once", text: "Acme ships this week", want: []string{"Acme"}},
		{name: "case insensitive", text: "acme ships this week", want: []string{"Acme"}},
		{name: "several occurrences are counted", text: "Acme and Acme again", want: []string{"Acme", "Acme"}},
		{name: "term as written is reported", text: "northwind wins", want: []string{"Northwind"}},
		{name: "absent term contributes nothing", text: "nothing to see", want: nil},
		{name: "occurrences keep source order", text: "Northwind then Acme", want: []string{"Northwind", "Acme"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tokens.Extract(test.text, keep)
			assertEqual(t, "names", got.Names, test.want)
		})
	}
}

func TestExtractWithoutGlossaryFindsNoNames(t *testing.T) {
	got := tokens.Extract("Acme is here", nil)
	if len(got.Names) != 0 {
		t.Fatalf("Names = %v, want none without a glossary", got.Names)
	}

	got = tokens.Extract("Acme is here", []string{"", "   "})
	if len(got.Names) != 0 {
		t.Fatalf("Names = %v, want none for blank glossary terms", got.Names)
	}
}

func TestExtractOnEmptyTextReturnsNothing(t *testing.T) {
	got := tokens.Extract("", []string{"Acme"})
	if len(got.Placeholders)+len(got.Markup)+len(got.Numbers)+len(got.Names) != 0 {
		t.Fatalf("Extract(\"\") = %+v, want an empty result", got)
	}
}

func TestDiffReportsMissingAndExtraValues(t *testing.T) {
	source := tokens.Extract("Hello %s, total 1,234.56 in <b>Acme</b>", []string{"Acme"})
	const target = "Hola {name}, total 12,345 en <i>Acme</i>"
	translated := tokens.Extract(target, []string{"Acme"})

	got := source.Diff(translated)
	want := []string{
		"markup extra: </i>",
		"markup extra: <i>",
		"markup missing: </b>",
		"markup missing: <b>",
		"number extra: 12,345",
		"number missing: 1,234.56",
		"placeholder extra: {name}",
		"placeholder missing: %s",
	}

	assertEqual(t, "diff", got, want)
}

func TestDiffCountsRepeatedValues(t *testing.T) {
	source := tokens.Extract("%s then %s", nil)
	translated := tokens.Extract("%s only", nil)

	got := source.Diff(translated)
	want := []string{"placeholder missing: %s"}

	assertEqual(t, "diff", got, want)

	reverse := translated.Diff(source)
	assertEqual(t, "reverse diff", reverse, []string{"placeholder extra: %s"})
}

func TestDiffIsEmptyWhenMultisetsMatch(t *testing.T) {
	source := tokens.Extract("Acme sent %s to {name} for 1,234.56 in <b>bold</b>", []string{"Acme"})
	translated := tokens.Extract("Acme envió %s a {name} por 1,234.56 en <b>negrita</b>", []string{"Acme"})

	if got := source.Diff(translated); len(got) != 0 {
		t.Fatalf("Diff() = %v, want an empty result", got)
	}
}

func TestDiffIgnoresOrder(t *testing.T) {
	source := tokens.Extract("%s and {name}", nil)
	translated := tokens.Extract("{name} y %s", nil)

	if got := source.Diff(translated); len(got) != 0 {
		t.Fatalf("Diff() = %v, want an empty result when only the order differs", got)
	}
}

func assertEqual(t *testing.T, label string, got, want []string) {
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

func TestExtractReportsEveryKindInOnePass(t *testing.T) {
	got := tokens.Extract("Acme: <b>1,234.56</b> for %s on {day}", []string{"Acme"})

	assertEqual(t, "placeholders", got.Placeholders, []string{"%s", "{day}"})
	assertEqual(t, "markup", got.Markup, []string{"<b>", "</b>"})
	assertEqual(t, "numbers", got.Numbers, []string{"1,234.56"})
	assertEqual(t, "names", got.Names, []string{"Acme"})
}

func TestDiffLinesAreSortedAndStable(t *testing.T) {
	source := tokens.Extract("", nil)
	translated := tokens.Extract("<b>{n}%s7</b>", nil)

	first := source.Diff(translated)
	second := source.Diff(translated)

	if len(first) != 5 {
		t.Fatalf("Diff() = %v, want five extra entries", first)
	}
	if strings.Join(first, "|") != strings.Join(second, "|") {
		t.Fatalf("Diff() is not stable: %v then %v", first, second)
	}
	for index := 1; index < len(first); index++ {
		if first[index-1] > first[index] {
			t.Fatalf("Diff() is not sorted: %v", first)
		}
	}
}
