package engine

import (
	"testing"
)

func TestSanitizeK8sName_Lowercase(t *testing.T) {
	got := sanitizeK8sName("MARKOV-RUN-RHAIRFE-100", 63)
	if got != "markov-run-rhairfe-100" {
		t.Errorf("got %q, want lowercase", got)
	}
}

func TestSanitizeK8sName_UnderscoresToDashes(t *testing.T) {
	got := sanitizeK8sName("process_tickets", 63)
	if got != "process-tickets" {
		t.Errorf("got %q, want process-tickets", got)
	}
}

func TestSanitizeK8sName_TruncatesWithHash(t *testing.T) {
	long := "markov-markov-run-8b10347b-process-tickets-rhairfe-100-rfe-speedrun"
	got := sanitizeK8sName(long, 63)
	if len(got) > 63 {
		t.Errorf("len = %d, want <= 63: %q", len(got), got)
	}
	if len(got) < 10 {
		t.Errorf("too short: %q", got)
	}
}

func TestSanitizeK8sName_ShortUnchanged(t *testing.T) {
	got := sanitizeK8sName("markov-run-abc-step1", 63)
	if got != "markov-run-abc-step1" {
		t.Errorf("got %q, want markov-run-abc-step1", got)
	}
}

func TestSanitizeK8sName_InvalidCharsRemoved(t *testing.T) {
	got := sanitizeK8sName("run@123!test", 63)
	for _, c := range got {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
			t.Errorf("invalid char %q in %q", string(c), got)
		}
	}
}

func TestSanitizeK8sName_NoLeadingTrailingDashes(t *testing.T) {
	got := sanitizeK8sName("-abc-def-", 63)
	if got[0] == '-' || got[len(got)-1] == '-' {
		t.Errorf("leading/trailing dash: %q", got)
	}
}

func TestSanitizeK8sName_RealisticJobName(t *testing.T) {
	raw := "markov-markov-run-8b10347b-process_tickets-RHAIRFE-100-rfe_speedrun"
	got := sanitizeK8sName(raw, 63)

	if len(got) > 63 {
		t.Errorf("len = %d, want <= 63: %q", len(got), got)
	}
	for _, c := range got {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
			t.Errorf("invalid char %q in %q", string(c), got)
		}
	}
	if got[0] == '-' || got[len(got)-1] == '-' {
		t.Errorf("leading/trailing dash: %q", got)
	}
}

func TestSanitizeK8sLabel_Truncates(t *testing.T) {
	long := "markov-run-8b10347b-process_tickets-RHAIRFE-100-rfe_speedrun-extra-stuff"
	got := sanitizeK8sLabel(long, 63)
	if len(got) > 63 {
		t.Errorf("len = %d, want <= 63: %q", len(got), got)
	}
}
