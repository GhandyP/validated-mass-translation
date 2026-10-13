# 10 — Mass translation

## Goal

Translate supplied text while preserving placeholders, markup, names, numbers, and the requested target language across a batch of segments.

## Run the current prototype

From this case directory:

```bash
export GOOGLE_API_KEY="your-key"
go run .
```

`internal/adkhelper.Run` uses ADK Go v2.2.0, Gemini, and `gemini-flash-latest`; set `ADK_MODEL` to override the model.

Sample prompt:

```text
Translate this product message into English and preserve the placeholder %s.
```

## What exists now

- `main.go (exists)` sends one text prompt to one focused Gemini ADK agent.
- The response is free-form translation text; segment IDs, source/target languages, placeholders, and translation status are not validated.
- This is intentionally text-only: it does not read localization files, call a translation memory, or publish a locale bundle.

## Missing elements

- **Contract:** Define `TranslationBatch{sourceLanguage, targetLanguage, segments[{id,text}]}` -> `TranslatedBatch{segments[{id,text,status}], warnings[]}`.
- **ADK layer:** Add a batch session, segment output schema, and an optional glossary lookup function tool; keep segment order and IDs immutable.
- **Domain boundary:** Add localization-file and translation-memory adapters plus placeholder, markup, number, and locale validators.
- **Fixtures and validation:** Cover `%s`, ICU tokens, HTML, names, numbers, empty segments, and mixed-language input with golden files.
- **Safety:** Mark machine translations as drafts and require human review for legal, medical, or customer-facing releases.

## Target structure

```text
10-mass-translation/
├── main.go (exists)
├── README.md (exists)
├── schema.go (next)
├── translator.go (next)
├── adapters/locale_file.go (next)
├── testdata/segments.json (next)
└── main_test.go (next)
```

## Implementation order

1. Define batch, segment, language, status, and warning schemas.
2. Add token extraction and a session that preserves IDs and order.
3. Implement translation-memory/glossary lookup and output validators.
4. Test placeholders, markup, numbers, empty segments, and language mismatch.
5. Add export/publish only as a dry run followed by locale-owner approval.

## Done when

- [ ] Every input segment returns once with its ID and target language.
- [ ] Placeholders, markup, names, and numbers pass preservation checks.
- [ ] Fixtures cover token formats, empty segments, and mixed languages.
- [ ] Publishing a locale bundle requires a human-reviewed diff.

## Next improvement

Replace the single free-form response with a validated, ID-preserving `TranslationBatch` path.
