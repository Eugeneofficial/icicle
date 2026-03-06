package ui

import "testing"

func TestBarHandlesNegativeWidth(t *testing.T) {
	theme := Theme{NoColor: true}
	got := theme.Bar(0.5, -3)
	if got != "" {
		t.Fatalf("expected empty bar for negative width, got %q", got)
	}
}

func TestBarUsesRequestedWidth(t *testing.T) {
	theme := Theme{NoColor: true}
	got := theme.Bar(0.5, 4)
	if got != "##  " {
		t.Fatalf("expected fixed-width ASCII bar, got %q", got)
	}
}
