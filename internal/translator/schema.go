package translator

import "google.golang.org/genai"

// Schema is the output schema the model must satisfy: one object holding an array
// of segments, each repeating an input id and carrying its translation.
//
// It deliberately has no status, language, or warning property. Those belong to
// Go, and a model must not be able to claim a review that never happened.
// schema_test.go fails if this schema and translation.Candidate ever disagree.
func Schema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"segments": {
				Type:        genai.TypeArray,
				Description: "One entry per input segment, in the input order.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id": {
							Type:        genai.TypeString,
							Description: "The id of the input segment this entry translates, copied exactly.",
						},
						"text": {
							Type:        genai.TypeString,
							Description: "The translated text, with every placeholder, tag, number, and glossary term preserved.",
						},
					},
					Required: []string{"id", "text"},
				},
			},
		},
		Required: []string{"segments"},
	}
}
