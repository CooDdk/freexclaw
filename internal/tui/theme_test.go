package tui

import (
	"strings"
	"testing"
)

func TestBrandSpinnerFrames_HasFourFrames(t *testing.T) {
	if got := len(BrandSpinnerFrames); got != 4 {
		t.Fatalf("expected 4 brand spinner frames, got %d", got)
	}
}

func TestBrandSpinnerFrames_AllNonEmpty(t *testing.T) {
	for i, f := range BrandSpinnerFrames {
		if strings.TrimSpace(f) == "" {
			t.Fatalf("frame %d is empty", i)
		}
	}
}

func TestMarkerUser_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerUser(), "❯") {
		t.Fatalf("expected MarkerUser to contain ❯, got %q", MarkerUser())
	}
}

func TestMarkerAssistant_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerAssistant(), "✻") {
		t.Fatalf("expected MarkerAssistant to contain ✻, got %q", MarkerAssistant())
	}
}

func TestMarkerToolStart_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerToolStart(), "▸") {
		t.Fatalf("expected MarkerToolStart to contain ▸, got %q", MarkerToolStart())
	}
}

func TestMarkerToolOK_ContainsCheck(t *testing.T) {
	if !strings.Contains(MarkerToolOK(), "✓") {
		t.Fatalf("expected MarkerToolOK to contain ✓, got %q", MarkerToolOK())
	}
}

func TestMarkerToolFail_ContainsCross(t *testing.T) {
	if !strings.Contains(MarkerToolFail(), "✗") {
		t.Fatalf("expected MarkerToolFail to contain ✗, got %q", MarkerToolFail())
	}
}

func TestMarkerWarn_ContainsSymbol(t *testing.T) {
	if !strings.Contains(MarkerWarn(), "⚠") {
		t.Fatalf("expected MarkerWarn to contain ⚠, got %q", MarkerWarn())
	}
}
