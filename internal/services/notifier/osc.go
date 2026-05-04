// Package notifier implements terminal-channel notification dispatch.
//
// This file constructs the raw byte sequences used to talk to terminals:
// OSC payloads, the BEL/ST terminators, and the tmux/screen DCS passthrough
// wrapper. The conventions mirror src/ink/termio/osc.ts so that the Go
// implementation produces byte-for-byte the same sequences as the TS source.
package notifier

import "os"

// OSC command numbers used for notifications. Mirrors src/ink/termio/osc.ts OSC.
const (
	OSCITerm2  = "9"   // iTerm2 proprietary sequences
	OSCKitty   = "99"  // Kitty notification protocol
	OSCGhostty = "777" // Ghostty notification protocol
)

// Control bytes.
const (
	ESC = "\x1b"
	BEL = "\x07"
	// ST is the String Terminator (ESC \). Used in place of BEL when an OSC
	// terminator is required by Kitty (avoids audible bell side effects).
	ST = ESC + `\`
	// OSCPrefix is the standard OSC introducer (ESC ]).
	OSCPrefix = ESC + "]"
)

// SEP is the OSC parameter separator.
const SEP = ";"

// buildOSC assembles an OSC sequence: ESC ] p1;p2;...;pN <terminator>.
//
// Use ST as the terminator for Kitty (matches the TS osc() helper which
// switches based on env.terminal === 'kitty'); use BEL for iTerm2 and Ghostty.
func buildOSC(terminator string, parts ...string) string {
	out := OSCPrefix
	for i, p := range parts {
		if i > 0 {
			out += SEP
		}
		out += p
	}
	out += terminator
	return out
}

// wrapForMultiplexer wraps an escape sequence for terminal multiplexer
// passthrough. tmux and GNU screen intercept escape sequences; DCS passthrough
// tunnels them to the outer terminal unmodified.
//
// tmux: ESC P tmux ; <escaped> ESC \, where every ESC inside the inner
// payload must be doubled (\x1b → \x1b\x1b).
//
// screen: ESC P <inner> ESC \ (no doubling).
//
// Outside tmux/screen, the sequence is returned unchanged.
//
// Do NOT wrap BEL through this helper: a raw \x07 triggers tmux's bell-action
// (window flag); wrapped \x07 becomes opaque DCS payload and tmux never sees
// the bell.
func wrapForMultiplexer(sequence string) string {
	if os.Getenv("TMUX") != "" {
		escaped := doubleESC(sequence)
		return ESC + "Ptmux;" + escaped + ESC + `\`
	}
	if os.Getenv("STY") != "" {
		return ESC + "P" + sequence + ESC + `\`
	}
	return sequence
}

// doubleESC replaces every ESC byte with two ESC bytes, matching the TS
// String.replaceAll('\x1b', '\x1b\x1b') used by tmux passthrough.
func doubleESC(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			out = append(out, 0x1b, 0x1b)
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
