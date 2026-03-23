package main

import (
	"strings"
	"testing"
)

// ─── getStyleTemplate ────────────────────────────────────────────────────────

func TestGetStyleTemplate_AllStyles(t *testing.T) {
	styles := []string{"legal", "technical", "report", "minimal", "letter"}
	for _, s := range styles {
		tmpl := getStyleTemplate(s)
		if tmpl == "" {
			t.Errorf("getStyleTemplate(%q) returned empty string", s)
		}
		// All templates should contain documentclass
		if !strings.Contains(tmpl, `\documentclass`) {
			t.Errorf("getStyleTemplate(%q) missing \\documentclass", s)
		}
	}
}

func TestGetStyleTemplate_UnknownDefaultsToReport(t *testing.T) {
	report := getStyleTemplate("report")
	unknown := getStyleTemplate("unknown")
	if report != unknown {
		t.Error("unknown style should fall back to report")
	}
}

func TestGetStyleTemplate_EmptyDefaultsToReport(t *testing.T) {
	report := getStyleTemplate("report")
	empty := getStyleTemplate("")
	if report != empty {
		t.Error("empty style should fall back to report")
	}
}

func TestGetStyleTemplate_ContainsRequiredColors(t *testing.T) {
	requiredColors := []string{"headingcolor", "rulecolor", "accentcolor", "codebg", "statusgreen", "statusamber", "statusblue"}
	styles := []string{"legal", "technical", "report", "minimal", "letter"}

	for _, s := range styles {
		tmpl := getStyleTemplate(s)
		for _, color := range requiredColors {
			if !strings.Contains(tmpl, color) {
				t.Errorf("getStyleTemplate(%q) missing required color %q", s, color)
			}
		}
	}
}

func TestGetStyleTemplate_ContainsFontSize(t *testing.T) {
	styles := []string{"legal", "technical", "report", "minimal", "letter"}
	for _, s := range styles {
		tmpl := getStyleTemplate(s)
		if !strings.Contains(tmpl, "FontSize") {
			t.Errorf("getStyleTemplate(%q) missing FontSize template variable", s)
		}
	}
}

func TestGetStyleTemplate_ContainsLastPage(t *testing.T) {
	styles := []string{"legal", "technical", "report", "minimal", "letter"}
	for _, s := range styles {
		tmpl := getStyleTemplate(s)
		if !strings.Contains(tmpl, "lastpage") {
			t.Errorf("getStyleTemplate(%q) missing lastpage package", s)
		}
	}
}

func TestGetStyleTemplate_UsesCorrectDelimiters(t *testing.T) {
	styles := []string{"legal", "technical", "report", "minimal", "letter"}
	for _, s := range styles {
		tmpl := getStyleTemplate(s)
		// Templates use << >> delimiters, should contain at least one
		if !strings.Contains(tmpl, "<<") || !strings.Contains(tmpl, ">>") {
			t.Errorf("getStyleTemplate(%q) should use << >> template delimiters", s)
		}
	}
}

// ─── statusColor ─────────────────────────────────────────────────────────────

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"FINAL", "statusgreen"},
		{"ACTIVE", "statusgreen"},
		{"DRAFT", "statusamber"},
		{"TEMPLATE", "statusblue"},
		{"UNKNOWN", "medgray"},
		{"", "medgray"},
		{"draft", "medgray"}, // case-sensitive
		{"Final", "medgray"},
	}

	for _, tt := range tests {
		got := statusColor(tt.status)
		if got != tt.expected {
			t.Errorf("statusColor(%q) = %q, want %q", tt.status, got, tt.expected)
		}
	}
}

// ─── classIcon ───────────────────────────────────────────────────────────────

func TestClassIcon(t *testing.T) {
	tests := []struct {
		class    string
		expected string
	}{
		{"CONFIDENTIAL", "CONFIDENTIAL"},
		{"INTERNAL", "INTERNAL USE ONLY"},
		{"PUBLIC", "PUBLIC"},
		{"UNKNOWN", ""},
		{"", ""},
		{"confidential", ""}, // case-sensitive
		{"Public", ""},
	}

	for _, tt := range tests {
		got := classIcon(tt.class)
		if got != tt.expected {
			t.Errorf("classIcon(%q) = %q, want %q", tt.class, got, tt.expected)
		}
	}
}
