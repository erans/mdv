package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// queryTerminal asks the terminal for the given OSC color codes (10 = default
// foreground, 11 = background, 4 = palette entries) and returns the answers
// keyed by code, with ANSI palette entries keyed as 100+index.
//
// ponytail: one write, one bounded read. Under tmux or a multiplexer that
// swallows OSC replies this simply returns false and the caller uses a
// built-in palette; upgrade to per-code retries only if that proves common.
func queryTerminal(codes []int) (map[int]string, bool) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, false
	}
	if err := os.Stdin.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		return nil, false // stdin is not pollable: never risk a blocking read
	}
	defer func() { _ = os.Stdin.SetReadDeadline(time.Time{}) }()

	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, false
	}
	defer func() { _ = term.Restore(fd, old) }()

	var q strings.Builder
	for _, code := range codes {
		if code == 4 {
			for i := 1; i <= 6; i++ { // red green yellow blue magenta cyan
				q.WriteString("\x1b]4;" + strconv.Itoa(i) + ";?\x07")
			}
			continue
		}
		q.WriteString("\x1b]" + strconv.Itoa(code) + ";?\x07")
	}
	if _, err := os.Stdout.WriteString(q.String()); err != nil {
		return nil, false
	}

	buf := make([]byte, 4096)
	n, _ := os.Stdin.Read(buf)
	ans := parseOSCColors(string(buf[:n]))
	if len(ans) == 0 {
		return nil, false
	}
	return ans, true
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
