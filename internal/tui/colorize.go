package tui

import "strings"

// colorizeLogLine applies GitHub Actions-aware coloring:
// dim ISO-8601 timestamp prefix, then style the rest by ##[level] marker
// (warning, error, group, notice, debug, command). Plain lines pass through.
func colorizeLogLine(line string) string {
	rest := line
	var ts string
	if i := timestampEnd(line); i > 0 {
		ts = line[:i]
		rest = line[i:]
	}
	out := logTimestampStyle.Render(ts)

	switch {
	case strings.HasPrefix(rest, "##[error]"):
		out += logErrorStyle.Render(rest)
	case strings.HasPrefix(rest, "##[warning]"):
		out += logWarningStyle.Render(rest)
	case strings.HasPrefix(rest, "##[group]"), strings.HasPrefix(rest, "##[endgroup]"):
		out += logGroupStyle.Render(rest)
	case strings.HasPrefix(rest, "##[notice]"):
		out += logNoticeStyle.Render(rest)
	case strings.HasPrefix(rest, "##[debug]"):
		out += logDebugStyle.Render(rest)
	case strings.HasPrefix(rest, "##[command]"), strings.HasPrefix(rest, "[command]"):
		out += logCommandStyle.Render(rest)
	default:
		out += rest
	}
	return out
}

// timestampEnd returns the byte index where a leading ISO-8601 timestamp
// (e.g. "2026-04-06T14:38:50.2346676Z ") ends, or 0 if none found.
func timestampEnd(s string) int {
	if len(s) < 20 {
		return 0
	}
	for i, c := range []byte("0000-00-00T00:00:00") {
		if c == '0' {
			if s[i] < '0' || s[i] > '9' {
				return 0
			}
		} else if s[i] != c {
			return 0
		}
	}
	i := 19
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i >= len(s) || s[i] != 'Z' {
		return 0
	}
	i++
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}
