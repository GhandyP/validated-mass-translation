package batch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Load reads exactly one strict JSON batch document from r.
//
// A document is rejected when it is empty, null, malformed, carries an unknown
// field, uses a wrong field type, or holds more than one JSON value. Every
// returned error wraps ErrInvalidBatch.
//
// Load checks the document, not the batch: it never reorders segments, never
// trims text, and leaves shape rules such as "at least one segment" to Validate.
func Load(r io.Reader) (TranslationBatch, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return TranslationBatch{}, fmt.Errorf("%w: read the batch document: %v", ErrInvalidBatch, err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return TranslationBatch{}, fmt.Errorf("%w: the batch document is empty", ErrInvalidBatch)
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return TranslationBatch{}, fmt.Errorf("%w: the batch document is null", ErrInvalidBatch)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()

	var loaded TranslationBatch
	if err := decoder.Decode(&loaded); err != nil {
		return TranslationBatch{}, fmt.Errorf("%w: decode the batch document: %v", ErrInvalidBatch, err)
	}

	var trailing json.RawMessage
	switch err := decoder.Decode(&trailing); {
	case err == nil:
		return TranslationBatch{}, fmt.Errorf("%w: the batch document contains more than one JSON value", ErrInvalidBatch)
	case errors.Is(err, io.EOF):
		// Exactly one JSON value: the only accepted shape.
	default:
		return TranslationBatch{}, fmt.Errorf("%w: trailing data after the batch document: %v", ErrInvalidBatch, err)
	}

	return loaded, nil
}
