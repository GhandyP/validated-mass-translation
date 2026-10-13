package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

const (
	// defaultModel is the model this repository is verified against.
	defaultModel = "gemini-flash-latest"
	// applicationName names the ADK application and its in-memory session service.
	applicationName = "validated-mass-translation"
	// translationAgentName names the agent inside the runner.
	translationAgentName = "batch_translator"
	// outputKey is the state key the structured answer is stored under.
	outputKey = "translated_batch"
)

// Gemini translates a batch with a schema-constrained Google ADK agent.
//
// The model is constructed once. Every Translate call builds its own agent and
// runner, because the instruction names that call's languages, and no call may
// observe another call's session state.
type Gemini struct {
	model model.LLM
}

var (
	_ Translator = (*Gemini)(nil)

	sessionSequence uint64
)

// NewGemini creates a translator backed by Gemini. It fails closed when
// GOOGLE_API_KEY is missing, so a caller never runs a half-configured model.
func NewGemini(ctx context.Context, opts Options) (*Gemini, error) {
	apiKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("%w: GOOGLE_API_KEY is not set", ErrModelUnavailable)
	}

	llm, err := gemini.NewModel(ctx, resolveModel(opts.Model), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("%w: create the Gemini model: %v", ErrModelUnavailable, err)
	}

	return &Gemini{model: llm}, nil
}

// Translate runs the agent once and returns its candidate.
//
// A batch that fails its own validation is returned as that batch error, so a
// caller can report an input problem as an input problem. Every other failure
// wraps ErrModelUnavailable.
func (g *Gemini) Translate(ctx context.Context, in batch.TranslationBatch, keep []string) (translation.Candidate, error) {
	if err := in.Validate(); err != nil {
		return translation.Candidate{}, err
	}
	if g == nil || g.model == nil {
		return translation.Candidate{}, fmt.Errorf("%w: the translator is not configured", ErrModelUnavailable)
	}

	glossary, err := GlossaryTool(keep)
	if err != nil {
		return translation.Candidate{}, fmt.Errorf("%w: build the glossary tool: %v", ErrModelUnavailable, err)
	}

	translator, err := llmagent.New(llmagent.Config{
		Name:         translationAgentName,
		Description:  "Translates a batch of segments while preserving every protected span.",
		Model:        g.model,
		Instruction:  Instruction(in),
		OutputSchema: Schema(),
		OutputKey:    outputKey,
		Tools:        []tool.Tool{glossary},
	})
	if err != nil {
		return translation.Candidate{}, fmt.Errorf("%w: create the translation agent: %v", ErrModelUnavailable, err)
	}

	r, err := runner.NewInMemory(applicationName, translator)
	if err != nil {
		return translation.Candidate{}, fmt.Errorf("%w: create the ADK runner: %v", ErrModelUnavailable, err)
	}

	message := &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: BatchPrompt(in, keep)}},
	}

	var output any
	for event, runErr := range r.Run(ctx, applicationName, nextSessionID(), message, agent.RunConfig{}) {
		if runErr != nil {
			return translation.Candidate{}, fmt.Errorf("%w: the ADK run failed: %v", ErrModelUnavailable, runErr)
		}
		if event != nil && event.Output != nil {
			output = event.Output
		}
	}
	if output == nil {
		return translation.Candidate{}, fmt.Errorf("%w: the model emitted no structured output", ErrModelUnavailable)
	}

	return decodeCandidate(output)
}

// decodeCandidate performs a JSON round trip, because ADK may hand back either a
// decoded map or an already-typed value.
func decodeCandidate(output any) (translation.Candidate, error) {
	encoded, err := json.Marshal(output)
	if err != nil {
		return translation.Candidate{}, fmt.Errorf("%w: marshal the structured output: %v", ErrModelUnavailable, err)
	}

	trimmed := strings.TrimSpace(string(encoded))
	if trimmed == "" || trimmed == "null" || !strings.HasPrefix(trimmed, "{") {
		return translation.Candidate{}, fmt.Errorf("%w: the structured output is not a JSON object", ErrModelUnavailable)
	}

	var candidate translation.Candidate
	if err := json.Unmarshal(encoded, &candidate); err != nil {
		return translation.Candidate{}, fmt.Errorf("%w: decode the structured output: %v", ErrModelUnavailable, err)
	}

	return candidate, nil
}

// resolveModel chooses a non-blank explicit model, then ADK_MODEL, then the
// stable default.
func resolveModel(override string) string {
	if model := strings.TrimSpace(override); model != "" {
		return model
	}
	if model := strings.TrimSpace(os.Getenv("ADK_MODEL")); model != "" {
		return model
	}

	return defaultModel
}

// nextSessionID returns a unique session per call, so no call can observe another
// call's conversation state or structured output.
func nextSessionID() string {
	return "batch-" + strconv.FormatUint(atomic.AddUint64(&sessionSequence, 1), 10)
}
