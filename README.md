# validated-mass-translation

A Go command that translates a batch of text segments with a schema-constrained Google ADK agent, and then refuses to hand back a translation that lost a placeholder, a tag, a number, a glossary term, an identifier, or the requested target language.

[![CI](https://github.com/GhandyP/validated-mass-translation/actions/workflows/ci.yml/badge.svg)](https://github.com/GhandyP/validated-mass-translation/actions/workflows/ci.yml)

## The idea

A structured-output schema guarantees *shape*, not *fidelity*. Asking a model to "keep the placeholders" is a request, not a guarantee: nothing stops `%s` from disappearing, `<b>` from turning into `<i>`, `1,234.56` from becoming `1.234,56`, or a segment from being merged with its neighbour.

So the model here is only allowed to return identifiers and text:

```json
{"segments": [{"id": "cart.title", "text": "Agregá %s al carrito"}]}
```

Everything else is decided by Go, not by the model: the draft status, both language tags, the warnings, the order, and whether the answer is acceptable at all. The validation is pure Go, fully offline, and covered by tests, so a plausible model answer fails loudly instead of shipping a broken locale bundle.

## Quickstart

This module uses the Go version declared in `go.mod`.

```bash
go build ./...
go test -race ./...
```

Those two commands need no API key and no network: they exercise the contract, the token scanner, the validation rules, the fake translator, and the CLI's golden files.

The translating commands construct the live Gemini translator and need `GOOGLE_API_KEY` exported:

```bash
export GOOGLE_API_KEY="your-key"

go run . -input fixtures/segments.json
cat fixtures/segments.json | go run . -input -
go run . -input fixtures/segments.json -out text
go run . -input fixtures/segments.json -keep Acme,Northwind
go run . -input fixtures/segments.json -locale path/to/es.json -dry-run
```

### Verification recorded for this checkout

The commands below were executed from the repository root with `GOOGLE_API_KEY` unset and with `GOPROXY=off`, so the local module cache was the only source of dependencies:

```text
$ gofmt -l .
(no output)

$ GOPROXY=off go build ./...
(no output, exit status 0)

$ GOPROXY=off go vet ./...
(no output, exit status 0)

$ GOPROXY=off go vet -tags=integration ./...
(no output, exit status 0)

$ GOPROXY=off go test -race -count=1 ./...
?   	github.com/GhandyP/validated-mass-translation	[no test files]
ok  	github.com/GhandyP/validated-mass-translation/internal/batch	1.037s
ok  	github.com/GhandyP/validated-mass-translation/internal/cli	1.194s
ok  	github.com/GhandyP/validated-mass-translation/internal/locale	1.084s
ok  	github.com/GhandyP/validated-mass-translation/internal/tokens	1.046s
ok  	github.com/GhandyP/validated-mass-translation/internal/translation	1.030s
ok  	github.com/GhandyP/validated-mass-translation/internal/translator	1.056s

$ GOPROXY=off go test -tags=integration ./internal/translator/
ok  	github.com/GhandyP/validated-mass-translation/internal/translator	0.011s
```

The offline suite holds 188 test and subtest pass lines, counted with `go test -count=1 -v`: 32 in `internal/batch`, 24 in `internal/cli`, 13 in `internal/locale`, 61 in `internal/tokens`, 38 in `internal/translation`, and 20 in `internal/translator`.

The tagged run reported `GOOGLE_API_KEY is not set; the live Gemini path is opt-in` and skipped. No key was available in this environment, so the commands that construct the Gemini translator were **not** executed here and this README makes no claim about a live model result.

## What it refuses to accept

Every rule has a stable code, so a failure names the segment and the reason instead of saying "something went wrong".

| Rule | What must hold | Code when it does not |
| --- | --- | --- |
| R1 | Every input segment comes back exactly once, matched by `id`. | `segment_missing`, `segment_duplicate`, `segment_unknown_id` |
| R2 | The answer keeps the input order. | `segment_out_of_order` |
| R3 | The batch echoes the target language the caller asked for; `-target-lang` refuses a batch that declares another one. | input error, exit status 2 |
| R4 | Placeholders survive as a multiset: `%s`, `%d`, `%1$s`, `%%`, `{name}`, and ICU expressions such as `{0, plural, one{# item} other{# items}}`. | `placeholder_missing`, `placeholder_extra` |
| R5 | Markup survives as written: HTML tags and entities. | `markup_missing`, `markup_extra` |
| R6 | Numbers survive as written, including thousands separators and decimals. | `number_missing`, `number_extra` |
| R7 | A segment that is empty or only whitespace stays empty. | `empty_segment_not_empty` |
| R8 | Every emitted segment is a draft, and a gated audience adds the release warning. | always set by Go |

Two details are worth knowing, because they are decisions rather than accidents:

- The placeholder and markup comparison is a **multiset**, not a set: dropping one of two identical `%s` placeholders fails.
- ICU expressions are compared **structurally**. The argument, the type, the option keys, and the nesting must survive, while whitespace and the branch text are not compared — because the text inside `one{...}` and `other{...}` is exactly what a translator is supposed to translate. A dropped expression, a renamed argument, a dropped option, and a dropped nested placeholder are all still caught.
- Glossary names come from an explicit `-keep` list, matched case-insensitively without word boundaries, because this program will not guess which words are names.

## Fixtures

| Fixture | What it covers |
| --- | --- |
| `fixtures/segments.json` | the main case: `%s`, a whitespace-only segment, `<b>` and `10%`, `1,234.56` |
| `fixtures/placeholders.json` | `%s`, `%d`, `%1$s`, `%%`, `{count}`, `{0, plural, ...}` |
| `fixtures/markup.json` | tags, a tag with attributes, entities (`&amp;`, `&#233;`) |
| `fixtures/names.json` | glossary names in a customer-facing batch |
| `fixtures/numbers.json` | thousands separators, decimals, negative and integer values |
| `fixtures/empty.json` | a whitespace-only segment |
| `fixtures/mixed-language.json` | a batch whose segments are not all in the source language |
| `fixtures/no-segments.json` | the input error case: a batch without segments |

The test suite asserts that these fixtures really cover those shapes (`TestFixturesCoverTheRequiredCases`), so the table above cannot drift away from the files.

## Output and exit statuses

`-out json` prints the translated batch (indented, with HTML left readable so a reviewer can see the markup), and `-out text` prints one line per segment with the text quoted, so a translation containing a line break cannot break the line-per-segment shape.

Warnings always go to standard error, never to standard output, so `-out json` stays pipeable:

```text
warning: machine translation: every segment is a draft
```

| Status | Meaning |
| --- | --- |
| 0 | success |
| 1 | model or runtime failure |
| 2 | input error: missing or unreadable input, malformed batch, empty batch, `-out` that is not `json` or `text`, `-target-lang` that contradicts the batch |
| 3 | validation failed: the candidate broke one of the rules above |
| 4 | publish refused: `-publish` without `-dry-run` |

## Publishing a locale bundle

This program never writes a locale file. Publishing a locale bundle requires a human-reviewed diff, so `-dry-run` prints the candidate bundle diff and the caller decides what to do with it:

```text
- cart.legacy: "Texto viejo"
+ cart.note: ""
~ cart.title: "Añadir %s al carrito" -> "Agregá %s al carrito"
```

`-publish` without `-dry-run` is refused with status 4, before any model call is spent. Every machine translation is marked `"status": "draft"`, and a batch with `"audience": "legal"`, `"medical"`, or `"customer-facing"` also carries:

```text
a human reviewer must approve legal, medical, or customer-facing releases before publication
```

## The Google ADK seam

Only `internal/translator` knows about Google ADK; everything else compares plain Go values. The stack is the same one the sibling published cases use, pinned in `go.mod`:

- `google.golang.org/adk/v2` v2.2.0
- `google.golang.org/genai` v1.66.0
- model `gemini-flash-latest`, overridable with `ADK_MODEL`

The agent is schema-constrained (`OutputSchema`), it runs on an in-memory runner with a unique session per call, and it can call a `lookup_glossary` function tool so the model can ask whether a term is protected instead of inferring it from the prompt:

```go
translator, err := llmagent.New(llmagent.Config{
    Name:         translationAgentName,
    Model:        g.model,
    Instruction:  Instruction(in),   // names this call's languages
    OutputSchema: Schema(),          // {"segments":[{"id","text"}]} and nothing else
    Tools:        []tool.Tool{glossary},
})
```

`NewGemini` fails closed when `GOOGLE_API_KEY` is missing, and the live path is proven only by an opt-in test:

```bash
go test -tags=integration ./internal/translator/
```

Without a key that test skips, so the offline suite never claims a live model result.

## Documentation

- `odd/tasks/validated-mass-translation.md` — the specification of record: the requirements (S1-S11), the
  frozen rules (R1-R8), the wire format, the command's contract, and the log of what was verified with which
  command.
- `docs/improvement-plan.md` — the reviewed backlog of what to improve next, with evidence, acceptance
  criteria, effort, and suggested batches. Nothing in it is implemented.

## License

MIT. See `LICENSE`.
