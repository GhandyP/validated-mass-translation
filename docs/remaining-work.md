# Remaining work — `validated-mass-translation`

**Status: open items only.** Written 2026-10-13 against `29cfb63`, the published tip.

This file is the short list of what is still missing and the order to attack it. `docs/improvement-plan.md`
is the detailed backlog: 28 items with `path:line` evidence, counterexamples, acceptance criteria, and
effort estimates. This file does not repeat that detail; it names the item IDs, adds the gaps the backlog
does not cover, and records the decisions that block the order.

It is **not** a specification. `odd/tasks/validated-mass-translation.md` is the specification of record,
and an accepted item moves there as a numbered `S#` plus a task before any code changes.

---

## 1. The only open gap that no offline command can close

**The live Gemini path has never executed.** No `GOOGLE_API_KEY` existed in the environment where this
repository was built, so every model-dependent claim is unproven: the ADK wiring, the schema-constrained
output, the model actually calling the glossary tool, and the prompt producing a candidate that survives
the rules.

To close it:

```bash
export GOOGLE_API_KEY="..."                            # never commit or paste this value anywhere
go run . -input fixtures/segments.json                 # expect exit 0 and a draft batch on stdout
go test -tags=integration ./internal/translator/ -v    # expect PASS, not SKIP
go run . -input fixtures/placeholders.json -out text
go run . -input fixtures/names.json -keep Acme,Northwind
```

Record the observed output in the README evidence block and in the ODD log. Delete the README's "not
executed here" sentence only once that output exists.

A live run failing is a finding, not a nuisance: keep the model's raw candidate beside the evidence, with
the problem codes it produced. The most likely and most informative first outcome is a failure on
`fixtures/placeholders.json`, because ICU expressions are where a model most often rewrites structure.

## 2. First code batch — closes real holes, all provable offline

Ordered so the cheap items land before the two that touch validation semantics. Item detail is in the
backlog.

| Order | Item | What it closes | Effort |
| --- | --- | --- | --- |
| 1 | M3 | `decodeCandidate`'s error branches have never executed | XS |
| 2 | M4 | nothing asserts `lookup_glossary` is still wired into the agent | XS |
| 3 | M5 | a malformed locale file never reaches exit status 2 in a test | XS |
| 4 | M6 | the README claims line-break safety with no test exercising it | XS |
| 5 | M2 | a swapped positional order (`%1$s %2$s`) passes validation | S |
| 6 | M1 | nothing verifies the answer is in the requested language; R3 is a tautology | S |
| 7 | M7 | the ODD document still contradicts itself about S10 | XS |
| 8 | M8 | the `audience` field is an unlogged deviation from the case contract | XS |

M1 is the only item here that changes a published claim, so it touches `README.md` as well; its wording
half should land even if the `LanguageChecker` seam is deferred.

## 3. The batch that changes what this project is

**S1** (`-candidate <path>`) and **S2** (machine-readable diagnostics). Today the thirteen rules can only
ever be applied to Gemini's own output, which contradicts the repository's own thesis that the value is in
the Go and not in the model. With S1 the same binary validates a translation produced by another vendor,
by a different tool, or by hand, with no API key and no network.

## 4. Gaps that are not code

| Item | What is missing | How it closes |
| --- | --- | --- |
| M11 | The Engram memory mirror and the session summary are filed under project `ghandy`, because the session started inside the unrelated `/home/ghandy` repository. | Open a session with this directory as the working directory, re-save the mirror and the summary under the repository's own project name, and confirm with `mem_context`. |
| S8 | CI has no `govulncheck`, no linter beyond `go vet`, no coverage report, and `actions/checkout@v4` plus `actions/setup-go@v5` carry a Node 20 deprecation annotation. | Add the steps and bump the actions after checking their current majors. |
| S9 | No tag and no release: nothing for a reader to pin or cite. | Tag `v0.1.0` at a verified tip, with release notes that state the verified scope and the unverified live path. |
| — | **No independent verification exists.** Every subagent anchor was refused in this environment, because the repository is its own Git clone nested inside the `/home/ghandy` clone, so the T8 verification was performed by the same agent that wrote the code. | An outside reader, a person or an agent working from a fresh `git clone`, re-runs the README's evidence block and reports disagreements. Until then, "verified" in this repository means "verified by its author". |
| S13 | Locale-format validation is absent, although the case asked for locale validators. | Declare it a non-goal, or implement a locale rule for number and date formatting with its own fixtures. |

## 5. Decisions that block the order

These are product choices, not defects. They are listed here because the batches above cannot be finished
coherently without an answer.

1. **Language.** Is "the answer is in the language I asked for" part of this tool's promise? If yes, M1
   grows a detector dependency and a new rule code; if no, the README and the ODD document must say
   plainly that only structure and protected spans are validated.
2. **Provenance of the candidate.** Is this a translation generator that validates itself (today), or a
   validator that accepts candidates from anywhere (S1)? The second is more useful, slightly more work,
   and it changes the repository's first paragraph.
3. **Scale and memory.** Is *mass* translation in scope? If yes, M9 needs chunking plus bounded retries,
   and translation memory must be either specified or declared out.
4. **Publishing.** The current rule is "never write a file, and refuse to publish without a reviewed
   diff". If a real workflow is wanted, the gate needs an approval artifact instead of a permanent
   refusal.

## 6. What is not missing

The independent repository and module, the thirteen rules with their codes, the offline suite and its
golden files, the command's contract with its five exit statuses, CI, the MIT license, the specification
and the process record, and the publication itself. All of it is committed and pushed; the commands that
prove it are in the README's evidence block and in the ODD log.

## How an item closes

Accept it into `odd/tasks/validated-mass-translation.md` as a numbered `S#` and a task, implement it with a
test that fails first, run the command that proves it, record the observed output, and delete the item
from `docs/improvement-plan.md`. An item is not closed until its evidence comes from a command that was
actually run.
