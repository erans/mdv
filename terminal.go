package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// probe is what one round trip to the terminal tells us: its colours and the
// pixel size of a cell. Asking twice would cost another timeout on every start,
// so the answers are gathered together and cached.
type probe struct {
	colors   map[int]string
	cellW    int
	cellH    int
	answered bool
}

var (
	probeOnce sync.Once
	probeVal  probe
)

// cellSizeFallback is what we assume when the terminal does not say.
const (
	fallbackCellW = 10
	fallbackCellH = 20
)

// probeTerminal asks for the default foreground and background, six ANSI hues,
// the cell size and the text area size, then keeps whatever came back.
//
// ponytail: one write, one bounded read. A multiplexer that swallows the replies
// just means the built-in palette and a 10x20 cell are used.
func probeTerminal() probe {
	probeOnce.Do(func() {
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return
		}
		if err := os.Stdin.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			return // stdin is not pollable: never risk a blocking read
		}
		defer func() { _ = os.Stdin.SetReadDeadline(time.Time{}) }()
		old, err := term.MakeRaw(fd)
		if err != nil {
			return
		}
		defer func() { _ = term.Restore(fd, old) }()

		var q strings.Builder
		q.WriteString("\x1b]11;?\x07\x1b]10;?\x07")
		for i := 1; i <= 6; i++ { // red green yellow blue magenta cyan
			q.WriteString("\x1b]4;" + strconv.Itoa(i) + ";?\x07")
		}
		q.WriteString("\x1b[16t") // cell size in pixels
		q.WriteString("\x1b[14t") // text area in pixels
		if strings.EqualFold(os.Getenv("TERM_PROGRAM"), "iTerm.app") {
			q.WriteString("\x1b]1337;ReportCellSize\x07")
		}
		if _, err := os.Stdout.WriteString(q.String()); err != nil {
			return
		}
		buf := make([]byte, 4096)
		n, _ := os.Stdin.Read(buf)
		reply := string(buf[:n])

		cols, rows := 0, 0
		if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			cols, rows = w, h
		}
		probeVal = probe{
			colors:   parseOSCColors(reply),
			answered: true,
		}
		probeVal.cellW, probeVal.cellH = cellSize(reply, cols, rows)
	})
	return probeVal
}

// cellSize reads the cell geometry out of a terminal reply: the direct `CSI 16 t`
// answer first, then iTerm2's OSC report, then the text area divided by the grid.
func cellSize(reply string, cols, rows int) (int, int) {
	if h, w, ok := parseCSITSize(reply, 6); ok && saneCell(w, h) {
		return w, h
	}
	if w, h, ok := parseOSCCellSize(reply); ok && saneCell(w, h) {
		return w, h
	}
	if h, w, ok := parseCSITSize(reply, 4); ok && cols > 0 && rows > 0 {
		cw, ch := w/cols, h/rows
		if saneCell(cw, ch) {
			return cw, ch
		}
	}
	return fallbackCellW, fallbackCellH
}

func saneCell(w, h int) bool {
	return w >= 3 && w <= 80 && h >= 6 && h <= 160
}

// parseCSITSize reads `CSI <code> ; height ; width t` replies: 6 is the cell
// size, 4 the text area.
func parseCSITSize(s string, code int) (h, w int, ok bool) {
	prefix := "\x1b[" + strconv.Itoa(code) + ";"
	i := strings.Index(s, prefix)
	if i < 0 {
		return 0, 0, false
	}
	rest := s[i+len(prefix):]
	end := strings.IndexByte(rest, 't')
	if end < 0 {
		return 0, 0, false
	}
	fields := strings.Split(rest[:end], ";")
	if len(fields) != 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(fields[0]))
	w, err2 := strconv.Atoi(strings.TrimSpace(fields[1]))
	if err1 != nil || err2 != nil || h <= 0 || w <= 0 {
		return 0, 0, false
	}
	return h, w, true
}

// parseOSCCellSize reads iTerm2's `OSC 1337;ReportCellSize=height;width;scale`.
func parseOSCCellSize(s string) (w, h int, ok bool) {
	const key = "ReportCellSize="
	i := strings.Index(s, key)
	if i < 0 {
		return 0, 0, false
	}
	rest := s[i+len(key):]
	if end := strings.IndexAny(rest, "\a\x1b"); end >= 0 {
		rest = rest[:end]
	}
	fields := strings.Split(rest, ";")
	if len(fields) < 2 {
		return 0, 0, false
	}
	fh, err1 := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
	fw, err2 := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
	if err1 != nil || err2 != nil || fh <= 0 || fw <= 0 {
		return 0, 0, false
	}
	return int(fw + 0.5), int(fh + 0.5), true
}

// parseOSCColors extracts `key;rgb:RR/GG/BB` replies for any terminator
// (BEL or ST) and any channel width.
func parseOSCColors(s string) map[int]string {
	out := map[int]string{}
	for {
		i := strings.Index(s, "\x1b]")
		if i < 0 {
			return out
		}
		s = s[i+2:]
		semi := strings.IndexByte(s, ';')
		end := strings.IndexAny(s, "\a")
		st := strings.Index(s, "\x1b\\")
		if st >= 0 && (end < 0 || st < end) {
			end = st
		}
		if semi < 0 || end < 0 || semi > end {
			continue
		}
		key, err := strconv.Atoi(s[:semi])
		if err != nil {
			continue
		}
		payload := s[semi+1 : end]
		s = s[end:]
		if key == 4 {
			// OSC 4 replies nest their index: `4;1;rgb:...`.
			if idx, p, ok := parsePaletteReply(payload); ok {
				out[100+idx] = p
			}
			continue
		}
		if hex, ok := parseRGBSpec(payload); ok {
			out[key] = hex
		}
	}
}

// parsePaletteReply splits an OSC 4 reply, which nests its own index.
func parsePaletteReply(payload string) (int, string, bool) {
	idxStr, spec, ok := strings.Cut(payload, ";")
	if !ok {
		return 0, "", false
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return 0, "", false
	}
	hex, ok := parseRGBSpec(spec)
	return idx, hex, ok
}

// parseRGBSpec converts `rgb:1a1b/1c1c/2626` to `#1a1b26`.
func parseRGBSpec(spec string) (string, bool) {
	spec, ok := strings.CutPrefix(spec, "rgb:")
	if !ok {
		return "", false
	}
	parts := strings.Split(spec, "/")
	if len(parts) != 3 {
		return "", false
	}
	var hex strings.Builder
	hex.WriteByte('#')
	for _, part := range parts {
		if part == "" || len(part) > 4 {
			return "", false
		}
		v, err := strconv.ParseUint(part, 16, 32)
		if err != nil {
			return "", false
		}
		switch len(part) {
		case 1:
			v *= 0x11
		case 2:
			// already 0..255
		case 3:
			v = v * 255 / 0xfff
		case 4:
			v >>= 8
		}
		hex.WriteString(hex2(int(v)))
	}
	return hex.String(), true
}
