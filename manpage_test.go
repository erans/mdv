package main

import (
	"strings"
	"testing"
)

// Every flag the completions know about has to be documented, or the manual
// drifts away from the binary that ships it.
func TestManPageDocumentsEveryFlag(t *testing.T) {
	for _, f := range completionFlags {
		if !strings.Contains(manPage, `\-`+f.name) {
			t.Errorf("man page does not document -%s", f.name)
		}
	}
	for _, section := range []string{".SH NAME", ".SH SYNOPSIS", ".SH OPTIONS", ".SH KEYS", ".SH THEME", ".SH CONFIG", ".SH MERMAID", ".SH IMAGES", ".SH ENVIRONMENT", ".SH FILES", ".SH EXAMPLES"} {
		if !strings.Contains(manPage, section) {
			t.Errorf("man page is missing %s", section)
		}
	}
}

func TestManPageIsValidRoff(t *testing.T) {
	if strings.Contains(manPage, "\t") {
		t.Error("a literal tab would confuse roff's quoting")
	}
	for i, line := range strings.Split(manPage, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// roff requests start with a dot; prose (and escaped dots) do not.
		if strings.HasPrefix(line, ".") && !strings.HasPrefix(line, ".\"") {
			fields := strings.Fields(line)
			if len(fields[0]) > 4 {
				t.Errorf("line %d: %q does not look like a roff request", i+1, fields[0])
			}
		}
	}
}

func TestCompletionsForEachShell(t *testing.T) {
	cases := map[string][]string{
		"bash": {"complete -o default -F _mdv mdv", "-images", "auto kitty iterm2 sixel halfblock off"},
		"zsh":  {"#compdef mdv", "_arguments", "_files"},
		"fish": {"complete -c mdv", "-o images", "complete -c mdv -F"},
	}
	for shell, wants := range cases {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		for _, want := range wants {
			if !strings.Contains(script, want) {
				t.Errorf("%s completion missing %q", shell, want)
			}
		}
	}
}

func TestCompletionsRejectUnknownShell(t *testing.T) {
	if _, err := completionScript("tcsh"); err == nil {
		t.Error("an unknown shell should be an error")
	}
}

// Flags that take a file argument should complete files, not options.
func TestCompletionsOfferImagesModes(t *testing.T) {
	script, _ := completionScript("zsh")
	if !strings.Contains(script, "mode:("+imageModes+")") {
		t.Error("zsh completion does not offer the image modes")
	}
	script, _ = completionScript("fish")
	if !strings.Contains(script, "-a '"+imageModes+"'") {
		t.Error("fish completion does not offer the image modes")
	}
}
