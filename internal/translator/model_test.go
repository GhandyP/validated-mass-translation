package translator

import "testing"

func TestResolveModelPrefersTheExplicitOverride(t *testing.T) {
	t.Setenv("ADK_MODEL", "from-the-environment")

	if got := resolveModel("explicit"); got != "explicit" {
		t.Fatalf("resolveModel() = %q, want the explicit override", got)
	}
}

func TestResolveModelFallsBackToTheEnvironment(t *testing.T) {
	t.Setenv("ADK_MODEL", "from-the-environment")

	if got := resolveModel("  "); got != "from-the-environment" {
		t.Fatalf("resolveModel() = %q, want the environment value", got)
	}
}

func TestResolveModelFallsBackToTheStableDefault(t *testing.T) {
	t.Setenv("ADK_MODEL", "")

	if got := resolveModel(""); got != defaultModel {
		t.Fatalf("resolveModel() = %q, want %q", got, defaultModel)
	}
}
