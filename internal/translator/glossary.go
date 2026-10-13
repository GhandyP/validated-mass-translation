package translator

import (
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// GlossaryToolName is the tool name the model sees.
const GlossaryToolName = "lookup_glossary"

// protectedNote tells the caller what a protected term requires.
const protectedNote = "copy this term into the translation character for character, never translate it"

// GlossaryMatch is the answer to one glossary question.
type GlossaryMatch struct {
	Term      string `json:"term" jsonschema:"the term that was looked up"`
	Protected bool   `json:"protected" jsonschema:"true when the term must be copied verbatim into the translation"`
	Note      string `json:"note" jsonschema:"what to do with the term; empty when the term is not protected"`
}

// glossaryArgs is the tool input: one term to look up.
type glossaryArgs struct {
	Term string `json:"term" jsonschema:"the term to look up in the protected glossary"`
}

// LookupGlossary reports whether term is one of the protected glossary terms.
//
// Matching ignores case and surrounding whitespace, and blank glossary entries are
// ignored, so a caller may pass an unfiltered list. The term is echoed as asked,
// so a model that wrote it differently still recognises the answer.
func LookupGlossary(keep []string, term string) GlossaryMatch {
	asked := strings.TrimSpace(term)
	match := GlossaryMatch{Term: asked}

	needle := strings.ToLower(asked)
	if needle == "" {
		return match
	}

	for _, entry := range keep {
		if strings.ToLower(strings.TrimSpace(entry)) == needle {
			match.Protected = true
			match.Note = protectedNote

			return match
		}
	}

	return match
}

// GlossaryTool exposes LookupGlossary to the agent, so the model can ask which
// terms have to survive verbatim instead of inferring it from the prompt alone.
func GlossaryTool(keep []string) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        GlossaryToolName,
		Description: "Looks up a term in the protected glossary and reports whether it must be copied verbatim.",
	}, func(_ agent.Context, args glossaryArgs) (GlossaryMatch, error) {
		return LookupGlossary(keep, args.Term), nil
	})
}
