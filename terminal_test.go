package main

import "testing"

func TestParseCSITSize(t *testing.T) {
	reply := "\x1b[6;20;10t"
	h, w, ok := parseCSITSize(reply, 6)
	if !ok || w != 10 || h != 20 {
		t.Fatalf("cell size = %dx%d ok=%v, want 10x20", w, h, ok)
	}
	if _, _, ok := parseCSITSize("\x1b[6;0;0t", 6); ok {
		t.Error("a zero size should be rejected")
	}
	if _, _, ok := parseCSITSize("\x1b[4;800;1200t", 6); ok {
		t.Error("code 4 must not be read as code 6")
	}
	if h, w, ok := parseCSITSize("\x1b[4;800;1200t", 4); !ok || h != 800 || w != 1200 {
		t.Errorf("text area = %dx%d ok=%v", w, h, ok)
	}
}

func TestParseOSCCellSize(t *testing.T) {
	for _, reply := range []string{
		"\x1b]1337;ReportCellSize=20.0;10.0;2.0\x07",
		"\x1b]1337;ReportCellSize=20.4;10.2\x1b\\",
	} {
		w, h, ok := parseOSCCellSize(reply)
		if !ok || w != 10 || h != 20 {
			t.Errorf("%q -> %dx%d ok=%v, want 10x20", reply, w, h, ok)
		}
	}
	if _, _, ok := parseOSCCellSize("nothing here"); ok {
		t.Error("garbage should not parse")
	}
}

// The direct cell answer wins; the text area divided by the grid is the backup.
func TestCellSizePrefersTheDirectAnswer(t *testing.T) {
	reply := "\x1b[6;2;1t" + "\x1b[4;800;1200t" // 2x1 px is not sane
	if w, h := cellSize(reply, 120, 40); w != 10 || h != 20 {
		t.Errorf("sane fallback from the text area = %dx%d, want 10x20", w, h)
	}
	reply = "\x1b[6;16;8t" + "\x1b[4;800;1200t"
	if w, h := cellSize(reply, 120, 40); w != 8 || h != 16 {
		t.Errorf("direct answer = %dx%d, want 8x16", w, h)
	}
	if w, h := cellSize("", 120, 40); w != 10 || h != 20 {
		t.Errorf("silent terminal = %dx%d, want the 10x20 assumption", w, h)
	}
}
