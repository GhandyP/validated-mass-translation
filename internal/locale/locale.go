// Package locale reads an existing locale bundle, renders the candidate bundle for
// a validated translation, and diffs the two so a human can review the change
// before anything is published.
//
// Nothing here writes a file: the diff is the artifact a reviewer reads.
package locale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/GhandyP/validated-mass-translation/internal/translation"
)

// Load reads a flat locale file: one JSON object mapping segment ids to text.
//
// A missing file is reported as an error wrapping os.ErrNotExist, so the caller
// decides whether an absent file means "first publish" or "wrong path".
func Load(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the locale file: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("the locale file %s is empty", path)
	}
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("the locale file %s is not a JSON object mapping ids to text", path)
	}

	bundle := make(map[string]string)
	if err := json.Unmarshal(trimmed, &bundle); err != nil {
		return nil, fmt.Errorf("decode the locale file %s: %w", path, err)
	}

	return bundle, nil
}

// Bundle renders the candidate locale bundle for a translated batch: one entry per
// segment id. Status and warnings are review metadata, not locale content, so they
// are not part of the bundle.
func Bundle(batch translation.TranslatedBatch) map[string]string {
	bundle := make(map[string]string, len(batch.Segments))
	for _, segment := range batch.Segments {
		bundle[segment.ID] = segment.Text
	}

	return bundle
}

// Diff renders the change from existing to candidate, one sorted line per
// difference: "- id: old" for a removal, "+ id: new" for an addition, and
// "~ id: old -> new" for a change. Unchanged entries are omitted, so an empty
// result means the candidate changes nothing.
func Diff(existing, candidate map[string]string) string {
	ids := make([]string, 0, len(existing)+len(candidate))
	recorded := make(map[string]struct{}, len(existing))

	for id := range existing {
		recorded[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range candidate {
		if _, seen := recorded[id]; !seen {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	var diff strings.Builder
	for _, id := range ids {
		before, existed := existing[id]
		after, exists := candidate[id]

		switch {
		case existed && !exists:
			diff.WriteString("- " + id + ": " + strconv.Quote(before) + "\n")
		case !existed && exists:
			diff.WriteString("+ " + id + ": " + strconv.Quote(after) + "\n")
		case existed && exists && before != after:
			diff.WriteString("~ " + id + ": " + strconv.Quote(before) + " -> " + strconv.Quote(after) + "\n")
		}
	}

	return diff.String()
}
