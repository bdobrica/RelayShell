package bridge

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ansiRenderer struct {
	line    []rune
	cursor  int
	pending string
}

func newANSIRenderer() *ansiRenderer {
	return &ansiRenderer{line: make([]rune, 0, 128)}
}

func (r *ansiRenderer) Feed(input string) []string {
	data := r.pending + input
	r.pending = ""

	lines := make([]string, 0)
	i := 0
	for i < len(data) {
		ch := data[i]

		switch ch {
		case '\n':
			lines = append(lines, r.flushLine())
			i++
		case '\r':
			r.cursor = 0
			i++
		case '\t':
			for t := 0; t < 4; t++ {
				r.writeRune(' ')
			}
			i++
		case '\b':
			if r.cursor > 0 {
				r.cursor--
			}
			i++
		case 0x1b:
			next, consumed, incomplete := r.consumeEscape(data[i:])
			if incomplete {
				r.pending = data[i:]
				return lines
			}
			i += consumed
			if next != 0 {
				// Reserved for future single-escape behaviors.
			}
		default:
			if !utf8.FullRuneInString(data[i:]) {
				r.pending = data[i:]
				return lines
			}
			rn, size := utf8.DecodeRuneInString(data[i:])
			i += size
			if rn == utf8.RuneError && size == 1 {
				continue
			}
			if rn >= 0x20 {
				r.writeRune(rn)
			}
		}
	}

	return lines
}

func (r *ansiRenderer) CurrentLine() string {
	return strings.TrimRight(string(r.line), " ")
}

func (r *ansiRenderer) FinalLine() string {
	r.pending = ""
	return r.CurrentLine()
}

func (r *ansiRenderer) consumeEscape(data string) (byte, int, bool) {
	if len(data) < 2 {
		return 0, 0, true
	}

	next := data[1]
	switch next {
	case ']':
		// OSC: ESC ] ... BEL or ESC \
		i := 2
		for i < len(data) {
			if data[i] == '\a' {
				return 0, i + 1, false
			}
			if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
				return 0, i + 2, false
			}
			i++
		}
		return 0, 0, true
	case '[':
		i := 2
		for i < len(data) {
			b := data[i]
			if b >= '@' && b <= '~' {
				params := data[2:i]
				r.applyCSI(params, b)
				return 0, i + 1, false
			}
			i++
		}
		return 0, 0, true
	default:
		return next, 2, false
	}
}

func (r *ansiRenderer) applyCSI(params string, final byte) {
	n := parseCSIParam(params, 1)
	switch final {
	case 'C':
		r.cursor += n
	case 'D':
		r.cursor -= n
		if r.cursor < 0 {
			r.cursor = 0
		}
	case 'G':
		if n > 0 {
			r.cursor = n - 1
		}
	case 'K':
		mode := parseCSIParam(params, 0)
		r.ensureCursor()
		switch mode {
		case 0:
			if r.cursor < len(r.line) {
				r.line = r.line[:r.cursor]
			}
		case 1:
			for j := 0; j < r.cursor && j < len(r.line); j++ {
				r.line[j] = ' '
			}
		case 2:
			r.line = r.line[:0]
			r.cursor = 0
		}
	case 'P':
		if r.cursor < len(r.line) {
			end := r.cursor + n
			if end > len(r.line) {
				end = len(r.line)
			}
			r.line = append(r.line[:r.cursor], r.line[end:]...)
		}
	case '@':
		r.ensureCursor()
		if n > 0 {
			insert := make([]rune, n)
			for j := range insert {
				insert[j] = ' '
			}
			r.line = append(r.line[:r.cursor], append(insert, r.line[r.cursor:]...)...)
		}
	case 'X':
		r.ensureCursor()
		for j := 0; j < n; j++ {
			pos := r.cursor + j
			if pos >= len(r.line) {
				break
			}
			r.line[pos] = ' '
		}
	case 'm', 'h', 'l', 's', 'u':
		// Formatting and mode toggles don't affect plain text rendering.
	case 'H', 'f':
		parts := strings.Split(params, ";")
		col := 1
		if len(parts) >= 2 {
			if parsed, err := strconv.Atoi(strings.TrimPrefix(parts[1], "?")); err == nil && parsed > 0 {
				col = parsed
			}
		}
		r.cursor = col - 1
	}
}

func (r *ansiRenderer) ensureCursor() {
	for len(r.line) < r.cursor {
		r.line = append(r.line, ' ')
	}
}

func (r *ansiRenderer) writeRune(ch rune) {
	r.ensureCursor()
	if r.cursor < len(r.line) {
		r.line[r.cursor] = ch
	} else {
		r.line = append(r.line, ch)
	}
	r.cursor++
}

func (r *ansiRenderer) flushLine() string {
	line := r.CurrentLine()
	r.line = r.line[:0]
	r.cursor = 0
	return line
}

func parseCSIParam(params string, defaultValue int) int {
	if params == "" {
		return defaultValue
	}
	part := params
	if idx := strings.IndexByte(params, ';'); idx >= 0 {
		part = params[:idx]
	}
	part = strings.TrimPrefix(part, "?")
	if part == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(part)
	if err != nil || n <= 0 {
		return defaultValue
	}
	return n
}

func renderBatchToLines(input string) []string {
	r := newANSIRenderer()
	lines := r.Feed(input)
	if tail := r.FinalLine(); tail != "" {
		lines = append(lines, tail)
	}
	return lines
}

func renderRawDebugToLines(input string) []string {
	parts := strings.Split(input, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, visualizeNonPrintable(part))
	}
	return lines
}

func visualizeNonPrintable(input string) string {
	var out strings.Builder
	for i := 0; i < len(input); {
		b := input[i]
		if b < 0x20 || b == 0x7f {
			appendByteCode(&out, b)
			i++
			continue
		}

		if b < 0x80 {
			out.WriteByte(b)
			i++
			continue
		}

		if !utf8.FullRuneInString(input[i:]) {
			appendByteCode(&out, b)
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(input[i:])
		if r == utf8.RuneError && size == 1 {
			appendByteCode(&out, b)
			i++
			continue
		}

		if !unicode.IsPrint(r) {
			appendRuneCode(&out, r)
			i += size
			continue
		}

		out.WriteString(input[i : i+size])
		i += size
	}

	return out.String()
}

func appendByteCode(out *strings.Builder, b byte) {
	out.WriteByte('<')
	hex := strings.ToUpper(strconv.FormatUint(uint64(b), 16))
	if len(hex) == 1 {
		out.WriteByte('0')
	}
	out.WriteString(hex)
	out.WriteByte('>')
}

func appendRuneCode(out *strings.Builder, r rune) {
	out.WriteString("<U+")
	hex := strings.ToUpper(strconv.FormatInt(int64(r), 16))
	out.WriteString(hex)
	out.WriteByte('>')
}
