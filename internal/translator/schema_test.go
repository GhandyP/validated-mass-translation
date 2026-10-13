package translator_test

import (
	"testing"

	"github.com/GhandyP/validated-mass-translation/internal/translator"
	"google.golang.org/genai"
)

func TestSchemaMirrorsTheCandidateContract(t *testing.T) {
	schema := translator.Schema()

	if schema.Type != genai.TypeObject {
		t.Fatalf("schema type = %v, want an object", schema.Type)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "segments" {
		t.Fatalf("schema required = %v, want [segments]", schema.Required)
	}

	segments, ok := schema.Properties["segments"]
	if !ok {
		t.Fatal("the schema has no segments property")
	}
	if segments.Type != genai.TypeArray {
		t.Fatalf("segments type = %v, want an array", segments.Type)
	}
	if segments.Items == nil || segments.Items.Type != genai.TypeObject {
		t.Fatalf("segments items = %+v, want an object", segments.Items)
	}
	if len(segments.Items.Required) != 2 || segments.Items.Required[0] != "id" || segments.Items.Required[1] != "text" {
		t.Fatalf("segment required = %v, want [id text]", segments.Items.Required)
	}
	if len(segments.Items.Properties) != 2 {
		t.Fatalf("segment properties = %v, want exactly id and text", segments.Items.Properties)
	}
	for _, name := range []string{"id", "text"} {
		property, ok := segments.Items.Properties[name]
		if !ok {
			t.Fatalf("the segment schema has no %s property", name)
		}
		if property.Type != genai.TypeString {
			t.Errorf("%s type = %v, want a string", name, property.Type)
		}
		if property.Description == "" {
			t.Errorf("%s has no description, which the model needs", name)
		}
	}
	if _, invented := segments.Items.Properties["status"]; invented {
		t.Error("the schema lets the model set status, which Go owns")
	}
}
