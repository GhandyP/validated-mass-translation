# Improvement plan — `validated-mass-translation`

**Status: proposal only. Nothing in this file is implemented.** It is the output of a read-only
review of the repository at commit `d3891a4`, the published tip.

This file is **not** a second specification. `odd/tasks/validated-mass-translation.md` is the
specification of record. An item is only implemented after it is accepted into that document as a
numbered `S#` and a task, following the same rule the repository already uses: one work unit per
task, tests beside the behavior, and evidence recorded from a command that was actually run.

## How to read this file

- **IDs**: `M` must fix (a defect or an unfulfilled contract), `S` should improve (structure or
  leverage), `C` could improve (polish). The numbers are stable references, not priority.
- **Effort**: XS under 30 minutes, S under 2 hours, M under a day, L more than a day, for one
  author with the repository already checked out.
- **Evidence** is quoted from the tree with `path:line`, so every claim in this file can be
  re-checked without trusting the author.
- **Done when** states the observable proof. An item is not done because the code changed; it is
  done when the named command prints the named result and that output is recorded.
- Unimplemented items stay here. Implemented items are deleted from here and appear in the ODD
  document as tasks. `M10` was closed by the commit that added this file and its number is not reused.

## Baseline: what is already verified today

Re-checking this list is the fastest way to notice that an item below was closed without recording
its evidence.

| Fact | Evidence |
| --- | --- |
| Independent repository and module | `git rev-parse --show-toplevel` prints the case directory; `go.mod` declares `google.golang.org/adk/v2 v2.2.0` and `google.golang.org/genai v1.66.0` with `go 1.26.5` |
| Formatting clean | `gofmt -l .` prints nothing |
| Build, vet, tagged vet | `GOPROXY=off go build ./...`, `go vet ./...`, `go vet -tags=integration ./...` each exit 0 |
| Offline suite green | `GOPROXY=off go test -race -count=1 ./...` green across six packages, 188 test and subtest pass lines |
| Live path opt-in and skipped | `go test -tags=integration ./internal/translator/` prints `GOOGLE_API_KEY is not set; the live Gemini path is opt-in` |
| Public exit statuses | through a built binary: `-h` 0, batch without segments 2, `-target-lang` contradiction 2, unknown flag 2, `-publish` without `-dry-run` 4 creating no file, valid batch without a key 1 |
| Never writes a file | zero `os.WriteFile`, `os.Create`, `os.OpenFile` calls in non-test code |
| Golden files are load-bearing | mutating either CLI golden made the test fail; restoring made it pass |
| Published | `https://github.com/GhandyP/validated-mass-translation` is `PUBLIC`, default branch `main`, CI green on the tip |

**Not verified, and no claim is made anywhere:** the live Gemini path. No `GOOGLE_API_KEY` existed
in the environment where this repository was built, so no model call was ever executed.

---

# A. Must fix

## M1 — Nothing verifies that the output is in the requested target language

- **Evidence**: `internal/translation/build.go` sets `TargetLanguage: in.TargetLanguage`, so the
  echo is written by Go and no candidate can ever fail rule R3. The only real R3 failure is the flag
  check in `internal/cli/run.go` (`-target-lang` against the batch). The R3 row of the rules table in
  `README.md` presents this as a validation.
- **Counterexample**: a model that answers in English for an `en` → `es` batch passes all eight
  rules; the batch is marked `draft` and the diff looks valid.
- **Why it must**: it is the largest gap between the thesis ("a schema guarantees shape, not
  fidelity; the Go decides") and the implementation. Of the properties a caller cares about, "it is
  in the language I asked for" is the first one.
- **Change**: two honest halves. (1) Declare it. Add to `README.md` and to the ODD document, as an
  explicit non-goal, that only structure, identifiers, order, and protected tokens are validated and
  that no language detection is performed. (2) Leave a seam: a `LanguageChecker` interface
  (`Check(segmentID, text string) error`) injected through `cli.Deps`, with an offline fake and a
  fake-driven test proving a foreign-language answer is refused with a stable code such as
  `target_language_mismatch`. A real detector can then be a later, separately authorized dependency.
- **Done when**: the README no longer implies R3 is a fidelity check, and a test shows a
  candidate that fails the injected checker produces exit status 3 with the new code.
- **Effort**: S for the seam and tests, XS for the wording.

## M2 — Positional printf arguments are compared as a multiset, so a swapped order passes

- **Evidence**: `internal/tokens/tokens.go`, `multisetDifference` counts occurrences in a map, so
  the comparison is order-insensitive; `scanPrintf` recognizes `%1$s` and keeps it as a span.
- **Counterexample**: source `%1$s owes %2$s`, candidate `%2$s owes %1$s`. Both have the multiset
  `{%1$s, %2$s}`, the difference is empty, and the batch is accepted, but the rendered sentence
  names the wrong parties.
- **Why it must**: positional verbs exist precisely because order is semantic. A validator that
  accepts a swap is wrong in the one case the syntax was invented for.
- **Change**: compare the ordered sequence of positional verbs separately from the unordered
  multiset of the rest. Report `placeholder_order` with both sequences in the detail.
- **Done when**: a test with the counterexample above fails with `placeholder_order`, and a test
  with an unchanged order still passes.
- **Effort**: S.

## M3 — `decodeCandidate` is a pure function with no test

- **Evidence**: `internal/translator/gemini.go:124` defines it; a repository-wide grep finds only
  the definition and its call at line 119. Its two rejection branches, "the structured output is not
  a JSON object" and "decode the structured output", have never executed.
- **Why it must**: it is the boundary between an untrusted model answer and typed Go values, and it
  is trivially testable offline. Untested error branches in a validator are where a fabricated
  success hides.
- **Change**: table-driven test over `nil`, `""`, `"null"`, a JSON array, a JSON object decoding
  into the wrong field types, and a valid object, asserting `errors.Is(err, ErrModelUnavailable)` and
  the surviving candidate.
- **Done when**: the test is green and covers every return statement of the function.
- **Effort**: XS.

## M4 — The glossary tool wiring is not asserted by any test

- **Evidence**: `internal/translator/schema_test.go` checks `Schema()` only. No test inspects
  `llmagent.Config`, so nothing proves `Tools: []tool.Tool{glossary}` still contains
  `lookup_glossary`.
- **Why it must**: deleting that line leaves every test green while the model silently loses the
  ability to resolve glossary terms, which is the one ADK feature the case's own specification asked
  for.
- **Change**: extract the config construction into a pure function (for example
  `agentConfig(in batch.TranslationBatch, llm model.LLM, keep []string) (llmagent.Config, error)`)
  and assert over its result: name, non-empty instruction, output schema identity, and a tool list
  containing `GlossaryToolName`. `Gemini.Translate` then calls it.
- **Done when**: a test fails if the tool is removed from the config.
- **Effort**: XS.

## M5 — The malformed locale file path through the CLI has no test

- **Evidence**: `internal/locale/locale_test.go` covers `locale.Load` thoroughly, but no test in
  `internal/cli/run_test.go` mentions `readExistingLocale`, so the branch that turns an unreadable or
  malformed locale file into exit status 2 is unverified.
- **Why it must**: it is part of the CLI's published contract (status 2), and it is the path a
  reviewer will exercise first with a hand-edited locale file.
- **Change**: add cases to the status-2 table: `-locale` pointing at a file with invalid JSON, at a
  file containing a JSON array, and at a directory. Assert exit status 2, an empty stdout, and a
  stderr that names the file.
- **Done when**: those three cases are in the table and green.
- **Effort**: XS.

## M6 — The README claims a behavior that no test exercises

- **Evidence**: `README.md` states that `-out text` quotes each translation "so a translation
  containing a line break cannot break the line-per-segment shape". No fixture or test has a
  translated text containing a newline: `internal/cli/testdata/segments.golden.txt` has none.
- **Why it must**: an unproven claim inside the evidence-driven document that is supposed to be the
  repository's credibility.
- **Change**: add a fixture segment whose fake translation contains a newline and a tab, and assert
  the text golden keeps exactly one line per segment with the escapes visible.
- **Done when**: the case exists, is green, and the golden shows the escape.
- **Effort**: XS.

## M7 — The ODD document contradicts itself about S10

- **Evidence**: `odd/tasks/validated-mass-translation.md` marks T9 done and prints the published URL
  and the verified equality of local `HEAD` and `origin/main`, while a later log line still says
  `S10 PENDING: the repository is not published yet; T9 is what proves it.`
- **Why it must**: the specification of record is the first thing a reviewer reads, and a
  self-contradicting record undermines the rest of the evidence.
- **Change**: replace the S10 line with the verifiable facts and the commands behind them
  (`gh repo view --json visibility`, `git rev-parse HEAD origin/main`, the CI run URL).
- **Done when**: the document has exactly one statement about S10 and it is true.
- **Effort**: XS.

## M8 — The `audience` field is an extension of the case contract that was never logged as a deviation

- **Evidence**: the frozen contract quoted in S2 is
  `TranslationBatch{sourceLanguage, targetLanguage, segments[{id,text}]}`. The implementation adds
  `Audience` (`internal/batch/batch.go:31`), used by the frozen wire format and by R8. The deviation
  list D1-D6 does not mention it.
- **Why it must**: the document's own audit trail claims to name every deviation. A field the
  caller must know about, presented without a deviation entry, is how a specification quietly drifts
  from the case it claims to implement.
- **Change**: add D7 explaining that `audience` is an addition, why it exists (it is the only input
  that can make the release warning testable), and that omitting it keeps the case contract valid.
- **Done when**: D7 exists and the frozen wire format references it.
- **Effort**: XS.

## M9 — Nothing scales: one prompt holds the whole batch, with no chunking, retries, or translation memory

- **Evidence**: `grep -rn "retry\|backoff\|chunk\|memory" internal/` matches only the ADK in-memory
  runner. `internal/translator/prompt.go` renders every segment of the batch into one prompt, and
  `Gemini.Translate` performs a single run with no retry. The case README's "Missing elements" asked
  for localization-file and translation-memory adapters; only a read-only locale adapter exists, and
  the translation memory was silently dropped from scope instead of being declared.
- **Why it must**: the project is named *mass* translation. With a few thousand segments the prompt
  exceeds the model's context, the run fails with a model error, and the caller gets exit status 1
  with no partial result and no resume. There is also no protection against a transient rate limit.
- **Change**: in priority order. (1) Chunk the batch by a configurable segment and character budget,
  keeping ids and order across chunks, and merge the candidates before `translation.Build`. (2)
  Retry a failed chunk with exponential backoff and jitter, bounded, and fail loudly when the budget
  is exhausted. (3) Declare translation memory and glossaries-as-lookup-tables as non-goals until
  they are specified, or specify them.
- **Done when**: a batch of at least 500 segments is translated as multiple chunks in one run, each
  chunk's failure is retried a bounded number of times, and the merged result still passes R1 and R2.
  Offline proof uses a fake translator that fails the first attempt of each chunk.
- **Effort**: M for chunking, S for retries.

## M11 — The memory mirror for this project is filed under the wrong project name

- **Evidence**: the Engram session in which this repository was built resolves to project
  `ghandy` because the working directory was, at session start, inside the unrelated `/home/ghandy`
  repository. The task mirror and the session summary had to be saved with `project: "ghandy"`.
- **Why it must**: a future session opened inside this repository will look for its continuity under
  `10-mass-translation` and find nothing, which is how a project loses its history.
- **Change**: open a session with this directory as the working directory and re-save the mirror and
  summary under the repository's own project name, then delete or supersede the mis-filed entries.
- **Done when**: `mem_context` inside this directory returns the mirror for this repository.
- **Effort**: XS.

---

# B. Should improve

## S1 — Let the validator run on a candidate that this program did not generate

- **Evidence**: `internal/cli/run.go` obtains the candidate only from `engine.Translate`, so the
  thirteen rules can only ever be applied to Gemini's output.
- **Why**: the thesis of the repository is that the value is in the Go, not in the model. As it
  stands, the Go cannot be used with any other provider, with a human-edited candidate, or in a
  pipeline that already has translations and only wants them checked.
- **Change**: add `-candidate <path|->`, a JSON document shaped like `translation.Candidate`. When
  it is present, skip the translator entirely. Keep `-dry-run`, the rules, the exit statuses, and the
  rendering unchanged.
- **Done when**: `go run . -input fixtures/segments.json -candidate fixtures/candidate.good.json`
  exits 0 and `-candidate fixtures/candidate.drifted.json` exits 3, both without an API key, with
  tests over those files.
- **Effort**: S.

## S2 — Machine-readable diagnostics

- **Evidence**: the stable codes surface only inside `ValidationError.Error()` prose on stderr.
  `Problem` already carries `Code`, `SegmentID`, and `Detail` and is already JSON-tagged, but nothing
  prints it.
- **Why**: a repository that advertises stable codes should let a pipeline consume them.
- **Change**: `-diagnostics json` (default unchanged) printing the problems array as JSON on stdout
  with the same exit statuses, or embedding them in the `-out json` document when validation fails.
- **Done when**: a golden test pins the JSON diagnostics document and the exit status stays 3.
- **Effort**: S.

## S3 — End-to-end coverage over every fixture

- **Evidence**: only `fixtures/segments.json` travels through the real CLI path. The other seven
  fixtures are exercised by `internal/batch` loading tests, so the token rules never run over the
  ICU, entity, name, and number fixtures through the command.
- **Why**: the fixtures were written to cover the rule shapes; today they cover only parsing.
- **Change**: a table-driven CLI test over every fixture with an injected fake whose answers are
  derived from the source text, asserting exit 0 for the rounded answers and exit 3 for deliberate
  drift per token kind.
- **Done when**: every fixture appears in the table and the table is green.
- **Effort**: S.

## S4 — Move the test double out of the production API

- **Evidence**: `Fake` is exported from `internal/translator/translator.go`, the package that is part
  of the shipped surface.
- **Why**: a fake in the public surface invites production code to depend on it, and it forces the
  package to keep an empty-body contract that only tests need.
- **Change**: move it to `internal/translator/translatortest` (or make it a test-only type) and
  update the golden tests to import the new package.
- **Done when**: `go doc ./internal/translator` no longer lists `Fake`, and the suite is green.
- **Effort**: XS.

## S5 — Nested ICU argument names are invisible to the comparator

- **Evidence**: `icuSignature` in `internal/tokens/tokens.go` copies braces from every depth but
  copies identifiers only at depth one, so a nested placeholder contributes `{}` with its name
  dropped. The behavior is documented in the function comment but not in the README rules table.
- **Why**: renaming or removing the argument of a nested placeholder changes the rendered message
  and passes validation.
- **Change**: recurse into nested expressions so their argument names and option keys join the
  signature, and add the limitation note to the README if recursion is deferred.
- **Done when**: a test with `{a, plural, other{{count} items}}` versus
  `{a, plural, other{{total} items}}` fails, and translated branch text still passes.
- **Effort**: S.

## S6 — Markup is compared as an opaque tag, attributes included

- **Evidence**: `scanMarkup` captures a whole tag as written, so `<a href="x" class="y">` and
  `<a class="y" href="x">` are different values, and a translatable attribute such as
  `alt="Add to cart"` must be copied character for character, which means it is never translated.
- **Why**: both effects are wrong in opposite directions: a legitimate attribute reorder fails, and
  the text a screen reader announces is left in the source language.
- **Change**: normalize attributes (sorted) for comparison, and treat the values of a documented
  set of translatable attributes (`alt`, `title`, `placeholder`, `aria-label`) as translatable text
  instead of markup. Record the decision in the ODD document, because it changes what R5 means.
- **Done when**: tests cover the reorder case as a pass and the untranslated `alt` case as the new
  documented behavior.
- **Effort**: M.

## S7 — Glossary matching has no word boundaries and folds case over byte offsets

- **Evidence**: `extractNames` lowercases the whole text and searches with `strings.Index`, with no
  boundary requirement (a documented decision), so a two-letter term such as `IT` matches inside
  `with`; and its own comment notes that a folded form of a different byte length could shift an
  ordering.
- **Why**: for a glossary, a false positive blocks a legitimate translation; for non-ASCII terms
  the ordering can be wrong.
- **Change**: match on runes with an optional boundary requirement (or a minimum length for
  unbounded matching), and fold case with a rune-safe helper while keeping byte offsets aligned.
- **Done when**: tests cover `IT` inside `with`, a Turkish-dotted term, and a term that legitimately
  appears twice.
- **Effort**: S.

## S8 — CI runs the happy path only

- **Evidence**: `.github/workflows/ci.yml` runs formatting, `go vet`, the tagged vet, the race
  suite, and the build. There is no `govulncheck`, no linter beyond vet, no coverage report, no
  matrix beyond the single `go.mod` version, and `actions/checkout@v4` plus `actions/setup-go@v5`
  carry a Node 20 deprecation annotation from the runner.
- **Why**: the repository's risk is dependency and drift, which is exactly what those checks cover;
  and a public example is judged partly by its pipeline.
- **Change**: add `govulncheck ./...`, a golangci-lint or staticcheck step, a coverage report as a
  build artifact, an OS matrix (ubuntu plus one other), and bump the two actions after checking
  their current major versions.
- **Done when**: the workflow is green with the new steps and the deprecation annotation is gone.
- **Effort**: S.

## S9 — No version and no release

- **Evidence**: `git tag -l` prints nothing.
- **Why**: a public example that a reader may want to pin or cite has no anchor.
- **Change**: tag `v0.1.0` at the verified tip with release notes that state the verified scope and
  the unverified live path.
- **Done when**: the tag exists and the release notes repeat those two facts.
- **Effort**: XS.

## S10 — The README does not point at the documents that matter

- **Evidence**: a grep for `odd/` and `docs/` in `README.md` returns nothing, although the
  specification and the process record live in `odd/tasks/validated-mass-translation.md`.
- **Why**: a reviewer cannot find the design source or the backlog, and the in-tree `odd/`
  directory looks unexplained.
- **Change**: a short documentation section listing `odd/tasks/validated-mass-translation.md` as the
  specification and `docs/improvement-plan.md` as the backlog.
- **Done when**: both links exist and resolve.
- **Effort**: XS.

## S11 — No data-handling or threat-model note

- **Evidence**: no section of `README.md` mentions privacy, PII, or prompt injection.
- **Why**: the command sends caller text to a third-party model, and the text of a segment is
  untrusted input that reaches the prompt. The structural defense is the validation, which today
  does not cover language (M1).
- **Change**: a section stating that segment text leaves the machine, that ids and text are only
  echoed inside the schema, that the output is validated rather than trusted, and that the current
  validation cannot detect content-level manipulation.
- **Done when**: the section exists and is consistent with the rules table.
- **Effort**: XS.

## S12 — The ODD document mixes specification with process log

- **Evidence**: `odd/tasks/validated-mass-translation.md` is over 300 lines; L9, L12, L14 and L15
  record the author's own tooling defects with as much weight as the requirements.
- **Why**: the specification is the part a future author must read before touching the code, and it
  is buried.
- **Change**: keep `odd/tasks/<feature>.md` as spec plus its `## Log` index, and move the narrative
  to an append-only `odd/log/<feature>.md`. Never edit history; append.
- **Done when**: the spec fits in one screen per section and the log is a separate file.
- **Effort**: S.

## S13 — Locale validation is absent although the case asked for it

- **Evidence**: the case README's missing elements include "placeholder, markup, number, and locale
  validators". The first three exist; locale reduces to echoing the requested tag.
- **Why**: locale is where number and date formatting actually live (`1,234.56` is right for `en-US`
  and wrong for `es-ES`), so the number rule and the locale rule are the same rule seen twice.
- **Change**: either declare locale-format validation a non-goal explicitly, or add a locale rule
  that checks the target tag's formatting conventions for the same numbers, keeping the locale
  expectations as fixtures.
- **Done when**: the ODD document says which of the two applies, with a test for whichever is chosen.
- **Effort**: M for the rule, XS for the declaration.

---

# C. Could improve

- **C1** — `CHANGELOG.md`, `CONTRIBUTING.md`, `SECURITY.md`, and issue and pull request templates.
  A public example is judged by whether a stranger can contribute without asking. Effort: S.
- **C2** — A `Makefile` or `justfile` with `check`, `test`, `lint`, and `evidence` targets, so CI and
  the README run the same commands instead of two lists that can drift. Effort: XS.
- **C3** — `-dry-run -format unified` producing a real unified diff, so the review step integrates
  with `git` and with review tools instead of a private `~ id: old -> new` format. Effort: XS.
- **C4** — Optional concurrency for chunked translation (M9), bounded and deterministic in output
  order, with `-race` tests. Effort: M.
- **C5** — A `examples/` directory with one runnable locale workflow end to end, including the
  candidate-review-candidate cycle implied by S1. Effort: S.

---

# Suggested batches

Each batch is independently valuable and independently releasable.

| Batch | Items | Why together | Effort |
| --- | --- | --- | --- |
| 1. Close the real holes | M1, M2, M3, M4, M5, M6 | All are small, all are validation or evidence defects, and all are provable offline with no key | ~1 day |
| 2. Honesty of the record | M7, M8, M10, S10, S11 | Documentation only; makes the published repository stop overstating itself | ~1 hour |
| 3. Change the category | S1, S2 | Turns the project from a Gemini demo into a translation validator usable by any pipeline | ~1 day |
| 4. Fidelity and scale | M9, S5, S6, S7 | These are the items that make the name `mass-translation` true and the preservation rules correct | ~2 days |
| 5. Pipeline and hygiene | S8, S9, C1, C2 | CI depth, a version anchor, and contributor basics | ~1 day |
| 6. Internal quality | S3, S4, S12, S13, C3, C4, C5 | Reduces the cost of the next six batches | ~2 days |

## Declared non-goals, so they stop reappearing as gaps

Unless a batch above is accepted, these stay out of scope and are stated as such in the README:
language detection and language-format validation (M1 keeps a seam only), publishing locale bundles
or any file write, translation memory and glossary persistence, multiple model providers, retry
budgets beyond the bounded chunk retries, and any human-approval workflow richer than the refusal
this program already performs.

---

## Provenance

- Written from a read-only review of the repository at `d3891a4`; no code was changed to produce it.
- Every `path:line` and every counterexample in this file was checked against that tree.
- The review that produced it was performed by the same agent that implemented the repository, so it
  is a self-review: it is a hypothesis list, not independent verification. An item's evidence should
  be re-checked before acceptance.
- Update rule: an accepted item moves into `odd/tasks/validated-mass-translation.md` as a numbered
  `S#` and a task, and is deleted from this file once its evidence is recorded.
