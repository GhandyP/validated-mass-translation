package translator

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
)

// instructionRules is the part of the instruction that does not depend on the
// batch. It is a plain constant, never a format string: the placeholders it names
// (%s, %1$s, %%) would otherwise be read as format directives.
const instructionRules = `Rules:
- Return exactly one segment for every input id, in the same order, with the same id.
- Never merge, split, reorder, rename, or invent a segment.
- Copy every placeholder character for character, including %s, %d, %1$s, %% and ICU expressions such as {name} or {0, plural, one{# item} other{# items}}.
- Copy every HTML tag and entity character for character, and translate only the text between them.
- Copy every number exactly as written, including separators: 1,234.56 stays 1,234.56.
- Copy every protected glossary term exactly as written, and call lookup_glossary when you are unsure whether a term is protected.
- Translate the segment text only: the ids are identifiers, not content.
- Return an empty text for a segment that has no text to translate.`

// Instruction is the system instruction for one batch. The language tags are
// quoted, so a tag cannot smuggle a line break or an extra instruction into the
// prompt.
func Instruction(in batch.TranslationBatch) string {
	return "You translate batches of user-interface text from " + strconv.Quote(in.SourceLanguage) +
		" into " + strconv.Quote(in.TargetLanguage) + ".\n\n" + instructionRules + "\n"
}

// BatchPrompt renders the model input for one batch: the languages, the protected
// glossary terms, and the segments as JSON objects, so a segment that is only
// whitespace stays visible instead of collapsing into an empty line.
func BatchPrompt(in batch.TranslationBatch, keep []string) string {
	var prompt strings.Builder

	prompt.WriteString("Translate these segments from " + strconv.Quote(in.SourceLanguage) +
		" into " + strconv.Quote(in.TargetLanguage) + ".\n")

	if terms := glossaryTerms(keep); len(terms) > 0 {
		prompt.WriteString("Protected glossary terms, to copy verbatim: " + strings.Join(terms, ", ") + "\n")
	}

	prompt.WriteString("Segments, one JSON object per line:\n")
	for _, segment := range in.Segments {
		// Marshaling a struct of two strings cannot fail; an unencodable segment
		// would be a programming error, not a runtime condition.
		encoded, err := json.Marshal(segment)
		if err != nil {
			continue
		}
		prompt.Write(encoded)
		prompt.WriteString("\n")
	}

	return prompt.String()
}

// glossaryTerms returns the non-blank glossary terms, first spelling wins, so the
// prompt lists each protected term exactly once.
func glossaryTerms(keep []string) []string {
	seen := make(map[string]struct{}, len(keep))
	terms := make([]string, 0, len(keep))

	for _, entry := range keep {
		term := strings.TrimSpace(entry)
		if term == "" {
			continue
		}
		key := strings.ToLower(term)
		if _, duplicated := seen[key]; duplicated {
			continue
		}
		seen[key] = struct{}{}
		terms = append(terms, term)
	}

	return terms
}
