// Package tokens extracts the spans a translation has to preserve: printf
// verbs, ICU expressions, HTML markup and entities, numbers, and glossary names.
//
// Every slice holds one entry per occurrence in source order, because callers
// compare multisets: a translation that drops one of two identical placeholders
// must fail, and a set comparison would hide exactly that bug.
package tokens

import (
	"sort"
	"strings"
)

// Tokens holds the protected spans found in one text.
type Tokens struct {
	Placeholders []string
	Markup       []string
	Numbers      []string
	Names        []string
}

// Extract scans text and returns the spans a translation has to preserve.
//
// keep is the glossary: the terms reported as written in keep, matched
// case-insensitively, left to right, one entry per non-overlapping occurrence.
// Word boundaries are not required, so "Acme" also matches inside "AcmeCorp";
// the glossary is an explicit caller-supplied list, so a substring match is the
// documented contract rather than a guess about which words are names.
//
// Spans are located in order and masked as they are found, so a value inside a
// protected span is never reported twice: caf&#233; contributes no number, and
// %1$s contributes no number.
//
// Malformed input never panics. An ICU expression without a closing brace is
// reported as a span running to the end of the text, because silently hiding a
// truncated placeholder is worse than reporting one the translator must keep. An
// unterminated tag or entity is ignored instead: it has no closing marker, so
// there is nothing to compare against.
func Extract(text string, keep []string) Tokens {
	mask := make([]bool, len(text))

	placeholders := append(scanPrintf(text, mask), scanICU(text, mask)...)

	return Tokens{
		Placeholders: valuesByStart(placeholders),
		Markup:       valuesByStart(scanMarkup(text, mask)),
		Numbers:      valuesByStart(scanNumbers(text, mask)),
		Names:        extractNames(text, keep),
	}
}

// Diff compares two token sets as multisets and returns one sorted line per
// difference, in the form "<kind> <missing|extra>: <value>".
//
// "missing" means the value is in the receiver and absent from other; "extra"
// means it is in other and absent from the receiver. A differing count of the
// same value is reported, and an order-only difference is no difference at all.
// An empty result therefore means the multisets match, which is the preservation
// check for placeholders, markup, numbers, and glossary names.
func (t Tokens) Diff(other Tokens) []string {
	kinds := []struct {
		kind string
		from []string
		to   []string
	}{
		{kind: "placeholder", from: t.Placeholders, to: other.Placeholders},
		{kind: "markup", from: t.Markup, to: other.Markup},
		{kind: "number", from: t.Numbers, to: other.Numbers},
		{kind: "name", from: t.Names, to: other.Names},
	}

	var lines []string
	for _, kind := range kinds {
		for _, value := range multisetDifference(kind.from, kind.to) {
			lines = append(lines, kind.kind+" missing: "+value)
		}
		for _, value := range multisetDifference(kind.to, kind.from) {
			lines = append(lines, kind.kind+" extra: "+value)
		}
	}

	sort.Strings(lines)
	return lines
}

// span is one located protected region of a text.
type span struct {
	start int
	end   int
	value string
}

// scanPrintf locates printf-style verbs: %s, %d, %v, positional %1$s, and any
// other single-letter verb. A doubled %% is an escape and is masked without being
// reported, and a percent sign that does not introduce a verb is left alone, so
// "10% off" is not a placeholder.
func scanPrintf(text string, mask []bool) []span {
	var spans []span

	for index := 0; index < len(text); index++ {
		if text[index] != '%' || mask[index] {
			continue
		}
		if index+1 < len(text) && text[index+1] == '%' {
			mask[index] = true
			mask[index+1] = true
			index++
			continue
		}

		cursor := index + 1
		for cursor < len(text) && isDigit(text[cursor]) {
			cursor++
		}
		if cursor < len(text) && text[cursor] == '$' {
			cursor++
		}
		if cursor >= len(text) || !isLetter(text[cursor]) {
			continue
		}
		cursor++

		spans = append(spans, span{start: index, end: cursor, value: text[index:cursor]})
		markRange(mask, index, cursor)
		index = cursor - 1
	}

	return spans
}

// scanICU locates ICU expressions: {name}, {0}, and plural or select forms with
// nested braces. An expression with no closing brace runs to the end of the text.
func scanICU(text string, mask []bool) []span {
	var spans []span

	for index := 0; index < len(text); index++ {
		if text[index] != '{' || mask[index] {
			continue
		}

		depth := 0
		end := -1
		for cursor := index; cursor < len(text) && end == -1; cursor++ {
			if mask[cursor] {
				continue
			}
			switch text[cursor] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = cursor + 1
				}
			}
		}
		if end == -1 {
			end = len(text)
		}

		spans = append(spans, span{start: index, end: end, value: text[index:end]})
		markRange(mask, index, end)
		index = end - 1
	}

	return spans
}

// scanMarkup locates HTML tags and entities as written. A less-than sign that
// does not open a tag is left alone, so "1 < 2" is not markup.
func scanMarkup(text string, mask []bool) []span {
	var spans []span

	for index := 0; index < len(text); index++ {
		if mask[index] {
			continue
		}

		switch text[index] {
		case '<':
			cursor := index + 1
			if cursor < len(text) && text[cursor] == '/' {
				cursor++
			}
			if cursor >= len(text) || !isLetter(text[cursor]) {
				continue
			}
			closing := strings.IndexByte(text[cursor:], '>')
			if closing < 0 {
				continue
			}
			end := cursor + closing + 1

			spans = append(spans, span{start: index, end: end, value: text[index:end]})
			markRange(mask, index, end)
			index = end - 1
		case '&':
			cursor := index + 1
			for cursor < len(text) && (isLetter(text[cursor]) || isDigit(text[cursor]) || text[cursor] == '#') {
				cursor++
			}
			if cursor == index+1 || cursor >= len(text) || text[cursor] != ';' {
				continue
			}
			end := cursor + 1

			spans = append(spans, span{start: index, end: end, value: text[index:end]})
			markRange(mask, index, end)
			index = end - 1
		}
	}

	return spans
}

// scanNumbers locates numeric literals as written, keeping thousands separators
// and decimals, and accepting a leading minus only where it reads as a sign:
// "2-3" reports 2 and 3, while "-7" reports -7. Masked regions are skipped, so the
// digits of an entity, a tag, an ICU expression, or a positional verb are not
// reported as numbers.
func scanNumbers(text string, mask []bool) []span {
	var spans []span

	for index := 0; index < len(text); index++ {
		if mask[index] || !isDigit(text[index]) {
			continue
		}

		start := index
		if start > 0 && text[start-1] == '-' && !mask[start-1] && (start < 2 || !isDigit(text[start-2])) {
			start--
		}

		cursor := index
		for cursor < len(text) && !mask[cursor] && isDigit(text[cursor]) {
			cursor++
		}
		for cursor+1 < len(text) && !mask[cursor] && text[cursor] == ',' && isDigit(text[cursor+1]) {
			cursor++
			for cursor < len(text) && !mask[cursor] && isDigit(text[cursor]) {
				cursor++
			}
		}
		if cursor+1 < len(text) && !mask[cursor] && text[cursor] == '.' && isDigit(text[cursor+1]) {
			cursor++
			for cursor < len(text) && !mask[cursor] && isDigit(text[cursor]) {
				cursor++
			}
		}

		spans = append(spans, span{start: start, end: cursor, value: text[start:cursor]})
		index = cursor - 1
	}

	return spans
}

// extractNames reports every occurrence of every glossary term, in the order the
// occurrences appear in the text. Positions come from a case-folded copy, so a
// term whose folded form has a different byte length than the term itself could
// shift an ordering compared with a reader's view; for the ASCII glossary terms
// this API is meant for, the two lengths agree.
func extractNames(text string, keep []string) []string {
	folded := strings.ToLower(text)

	var found []span
	for _, term := range keep {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			continue
		}
		needle := strings.ToLower(trimmed)

		for offset := 0; offset+len(needle) <= len(folded); {
			position := strings.Index(folded[offset:], needle)
			if position < 0 {
				break
			}
			start := offset + position
			found = append(found, span{start: start, end: start + len(needle), value: trimmed})
			offset = start + len(needle)
		}
	}

	return valuesByStart(found)
}

// multisetDifference returns the entries of from that outnumber to, one entry per
// surplus occurrence.
func multisetDifference(from, to []string) []string {
	surplus := make(map[string]int, len(from))
	for _, value := range from {
		surplus[value]++
	}
	for _, value := range to {
		surplus[value]--
	}

	var values []string
	for value, count := range surplus {
		for occurrence := 0; occurrence < count; occurrence++ {
			values = append(values, value)
		}
	}

	return values
}

// valuesByStart returns the span values in source order.
func valuesByStart(spans []span) []string {
	ordered := make([]span, len(spans))
	copy(ordered, spans)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].start < ordered[j].start })

	values := make([]string, 0, len(ordered))
	for _, item := range ordered {
		values = append(values, item.value)
	}

	return values
}

func markRange(mask []bool, start, end int) {
	for index := start; index < end && index < len(mask); index++ {
		mask[index] = true
	}
}

func isDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

func isLetter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}
