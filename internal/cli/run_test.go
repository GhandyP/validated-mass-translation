package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/batch"
	"github.com/GhandyP/validated-mass-translation/internal/cli"
	"github.com/GhandyP/validated-mass-translation/internal/translation"
)

const segmentsFixture = "../../fixtures/segments.json"

// stub records what the CLI asked for and answers from byID, so a test can drive
// both a rounded and a drifting translation without a model call.
type stub struct {
	byID  map[string]string
	fail  error
	calls int
	keep  []string
}

func (s *stub) Translate(_ context.Context, in batch.TranslationBatch, keep []string) (translation.Candidate, error) {
	s.calls++
	s.keep = keep
	if s.fail != nil {
		return translation.Candidate{}, s.fail
	}

	segments := make([]translation.CandidateSegment, 0, len(in.Segments))
	for _, segment := range in.Segments {
		segments = append(segments, translation.CandidateSegment{ID: segment.ID, Text: s.byID[segment.ID]})
	}

	return translation.Candidate{Segments: segments}, nil
}

// spanish answers fixtures/segments.json while preserving every protected span.
func spanish() *stub {
	return &stub{byID: map[string]string{
		"cart.title":   "Agregá %s al carrito",
		"cart.note":    "",
		"promo.banner": "Ahorrá <b>10%</b> en tu pedido",
		"cart.total":   "Total: 1,234.56 USD",
	}}
}

func run(t *testing.T, deps cli.Deps, stdin string, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr, deps)

	return code, stdout.String(), stderr.String()
}

func golden(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}

	return string(content)
}

func writeBatch(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "batch.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	return path
}

func TestRunWritesTheGoldenJSON(t *testing.T) {
	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture)

	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != golden(t, "segments.golden.json") {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, golden(t, "segments.golden.json"))
	}
	if stderr != "warning: machine translation: every segment is a draft\n" {
		t.Fatalf("stderr = %q, want the draft warning", stderr)
	}
}

func TestRunWritesTheGoldenText(t *testing.T) {
	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-out", "text")

	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != golden(t, "segments.golden.txt") {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, golden(t, "segments.golden.txt"))
	}
}

func TestRunReadsTheBatchFromStandardInput(t *testing.T) {
	content, err := os.ReadFile(segmentsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, string(content), "-input", "-")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != golden(t, "segments.golden.json") {
		t.Fatalf("stdin was not read: stdout =\n%s", stdout)
	}
}

func TestRunReportsInputProblemsAsStatusTwo(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantText string
	}{
		{name: "no input flag", args: nil, wantText: "input"},
		{name: "unknown flag", args: []string{"-input", segmentsFixture, "-invented"}, wantText: "invented"},
		{name: "unreadable input", args: []string{"-input", filepath.Join("/tmp", "absent-case10.json")}, wantText: "absent-case10.json"},
		{name: "unknown output format", args: []string{"-input", segmentsFixture, "-out", "yaml"}, wantText: "yaml"},
		{name: "target language contradicts the batch", args: []string{"-input", segmentsFixture, "-target-lang", "fr"}, wantText: "target"},
		{name: "empty batch", args: []string{"-input", "@empty"}, wantText: "segment"},
		{name: "malformed batch", args: []string{"-input", "@malformed"}, wantText: "decode"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := test.args
			switch {
			case len(args) == 2 && args[1] == "@empty":
				args = []string{"-input", writeBatch(t, "{\"sourceLanguage\":\"en\",\"targetLanguage\":\"es\",\"segments\":[]}")}
			case len(args) == 2 && args[1] == "@malformed":
				args = []string{"-input", writeBatch(t, "not json")}
			}

			code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, test.stdin, args...)
			if code != 2 {
				t.Fatalf("Run() = %d, want 2 (stdout: %s, stderr: %s)", code, stdout, stderr)
			}
			if !strings.Contains(stderr, test.wantText) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantText)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want nothing on an input error", stdout)
			}
		})
	}
}

func TestRunAcceptsTheTargetLanguageTheBatchDeclares(t *testing.T) {
	code, _, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-target-lang", "es")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
}

func TestRunReportsValidationFailureAsStatusThree(t *testing.T) {
	path := writeBatch(t, "{\"sourceLanguage\":\"en\",\"targetLanguage\":\"es\",\"segments\":[{\"id\":\"s\",\"text\":\"Add %s to the cart\"}]}")
	drifting := &stub{byID: map[string]string{"s": "Añadir al carrito"}}

	code, stdout, stderr := run(t, cli.Deps{Translator: drifting}, "", "-input", path)
	if code != 3 {
		t.Fatalf("Run() = %d, want 3 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "placeholder_missing") {
		t.Fatalf("stderr = %q, want the placeholder_missing code", stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing when validation fails", stdout)
	}
}

func TestRunReportsAModelFailureAsStatusOne(t *testing.T) {
	failure := errors.New("the quota is exhausted")

	code, stdout, stderr := run(t, cli.Deps{Translator: &stub{fail: failure}}, "", "-input", segmentsFixture)
	if code != 1 {
		t.Fatalf("Run() = %d, want 1 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "the quota is exhausted") {
		t.Fatalf("stderr = %q, want the model failure", stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing when the model fails", stdout)
	}
}

func TestRunFailsClosedWithoutATranslatorAndWithoutAnAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	code, _, stderr := run(t, cli.Deps{}, "", "-input", segmentsFixture)
	if code != 1 {
		t.Fatalf("Run() = %d, want 1 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "GOOGLE_API_KEY") {
		t.Fatalf("stderr = %q, want it to name the missing key", stderr)
	}
}

func TestRunRefusesToPublishWithoutADryRun(t *testing.T) {
	directory := t.TempDir()
	bundle := filepath.Join(directory, "es.json")

	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-locale", bundle, "-publish")
	if code != 4 {
		t.Fatalf("Run() = %d, want 4 (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "human-reviewed diff") {
		t.Fatalf("stderr = %q, want the human-reviewed diff requirement", stderr)
	}
	if _, err := os.Stat(bundle); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the program created %s, and it must never write a locale file", bundle)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing when publish is refused", stdout)
	}
}

func TestRunDryRunWritesTheGoldenDiffAndTouchesNothing(t *testing.T) {
	directory := t.TempDir()
	bundle := filepath.Join(directory, "es.json")
	existing := golden(t, "locale.existing.json")
	if err := os.WriteFile(bundle, []byte(existing), 0o600); err != nil {
		t.Fatalf("write existing locale: %v", err)
	}

	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-locale", bundle, "-dry-run", "-publish")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != golden(t, "publish.golden.txt") {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, golden(t, "publish.golden.txt"))
	}

	after, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatalf("read the locale file after the run: %v", err)
	}
	if string(after) != existing {
		t.Fatalf("the locale file changed:\n%s", after)
	}
}

func TestRunDryRunWithoutAPublishFlagPrintsTheSameDiff(t *testing.T) {
	directory := t.TempDir()
	bundle := filepath.Join(directory, "es.json")
	if err := os.WriteFile(bundle, []byte(golden(t, "locale.existing.json")), 0o600); err != nil {
		t.Fatalf("write existing locale: %v", err)
	}

	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-locale", bundle, "-dry-run")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != golden(t, "publish.golden.txt") {
		t.Fatalf("stdout =\n%s\nwant the publish diff", stdout)
	}
}

func TestRunDryRunWithNoExistingLocaleReportsAdditions(t *testing.T) {
	code, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-dry-run")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "+ cart.title: \"Agregá %s al carrito\"") {
		t.Fatalf("stdout =\n%s\nwant the first publish to read as additions", stdout)
	}
}

func TestRunPassesTheGlossaryToTheTranslator(t *testing.T) {
	recorder := spanish()

	code, _, stderr := run(t, cli.Deps{Translator: recorder}, "", "-input", segmentsFixture, "-keep", "Acme, ,acme,Northwind")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}
	if recorder.calls != 1 {
		t.Fatalf("the translator was called %d times, want exactly 1", recorder.calls)
	}
	if len(recorder.keep) != 2 || recorder.keep[0] != "Acme" || recorder.keep[1] != "Northwind" {
		t.Fatalf("keep = %v, want [Acme Northwind] with blanks and repeats removed", recorder.keep)
	}
}

func TestRunHelpPrintsUsageAndSucceeds(t *testing.T) {
	code, stdout, stderr := run(t, cli.Deps{}, "", "-h")
	if code != 0 {
		t.Fatalf("Run() = %d, want 0 (stderr: %s)", code, stderr)
	}

	for _, want := range []string{"-input", "-target-lang", "-keep", "-out", "-locale", "-dry-run", "-publish", "Exit statuses", "human-reviewed diff"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("usage does not mention %q", want)
		}
	}
}

func TestRunKeepsWarningsOutOfStandardOutput(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			_, stdout, stderr := run(t, cli.Deps{Translator: spanish()}, "", "-input", segmentsFixture, "-out", format)

			for _, line := range strings.Split(stdout, "\n") {
				if strings.HasPrefix(line, "warning") {
					t.Errorf("stdout carries a warning line: %q", line)
				}
			}
			if format == "json" {
				var parsed any
				if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
					t.Errorf("stdout is not pure JSON: %v", err)
				}
			}
			if !strings.Contains(stderr, "warning: ") {
				t.Errorf("stderr = %q, want the warning there", stderr)
			}
		})
	}
}
