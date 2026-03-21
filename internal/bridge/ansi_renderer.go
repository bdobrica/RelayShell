package bridge

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ansiScreen struct {
	rows map[int][]rune
	row  int
	col  int
}

func newANSIScreen() *ansiScreen {
	return &ansiScreen{
		rows: make(map[int][]rune),
	}
}

func (s *ansiScreen) writeRune(ch rune) {
	s.ensureRowCol()
	line := s.rows[s.row]
	if s.col < len(line) {
		line[s.col] = ch
	} else {
		line = append(line, ch)
	}
	s.rows[s.row] = line
	s.col++
}

func (s *ansiScreen) ensureRowCol() {
	if s.row < 0 {
		s.row = 0
	}
	if s.col < 0 {
		s.col = 0
	}
	line := s.rows[s.row]
	for len(line) < s.col {
		line = append(line, ' ')
	}
	s.rows[s.row] = line
}

func (s *ansiScreen) applyCSI(params string, final byte) {
	n := parseCSIParam(params, 1)
	switch final {
	case 'A':
		s.row -= n
		if s.row < 0 {
			s.row = 0
		}
	case 'B':
		s.row += n
	case 'C':
		s.col += n
	case 'D':
		s.col -= n
		if s.col < 0 {
			s.col = 0
		}
	case 'G':
		if n > 0 {
			s.col = n - 1
		}
	case 'H', 'f':
		row, col := parseCursorPos(params)
		s.row = row - 1
		s.col = col - 1
	case 'K':
		s.applyEraseInLine(parseCSIParam(params, 0))
	case 'J':
		s.applyEraseInDisplay(parseCSIParam(params, 0))
	case 'P':
		s.applyDeleteChars(n)
	case '@':
		s.applyInsertSpaces(n)
	case 'X':
		s.applyEraseChars(n)
	case 'h', 'l':
		if hasPrivateMode(params, 47) || hasPrivateMode(params, 1047) || hasPrivateMode(params, 1049) {
			s.reset()
		}
	case 'm', 's', 'u':
		// Formatting and mode toggles don't affect plain text rendering.
	}
}

func (s *ansiScreen) reset() {
	s.rows = make(map[int][]rune)
	s.row = 0
	s.col = 0
}

func (s *ansiScreen) applyEraseInLine(mode int) {
	s.ensureRowCol()
	line := s.rows[s.row]
	switch mode {
	case 0:
		if s.col < len(line) {
			line = line[:s.col]
		}
	case 1:
		for i := 0; i <= s.col && i < len(line); i++ {
			line[i] = ' '
		}
	case 2:
		line = line[:0]
		s.col = 0
	}
	s.rows[s.row] = line
}

func (s *ansiScreen) applyEraseInDisplay(mode int) {
	switch mode {
	case 2:
		s.reset()
	case 3:
		// Erase saved lines (scrollback) is treated as a full clear for chat rendering.
		s.reset()
	case 0:
		s.applyEraseInLine(0)
		for r := s.row + 1; ; r++ {
			if _, ok := s.rows[r]; !ok {
				break
			}
			delete(s.rows, r)
		}
	case 1:
		s.applyEraseInLine(1)
		for r := 0; r < s.row; r++ {
			delete(s.rows, r)
		}
	}
}

func (s *ansiScreen) applyDeleteChars(n int) {
	if n <= 0 {
		return
	}
	s.ensureRowCol()
	line := s.rows[s.row]
	if s.col >= len(line) {
		return
	}
	end := s.col + n
	if end > len(line) {
		end = len(line)
	}
	line = append(line[:s.col], line[end:]...)
	s.rows[s.row] = line
}

func (s *ansiScreen) applyInsertSpaces(n int) {
	if n <= 0 {
		return
	}
	s.ensureRowCol()
	line := s.rows[s.row]
	insert := make([]rune, n)
	for i := range insert {
		insert[i] = ' '
	}
	line = append(line[:s.col], append(insert, line[s.col:]...)...)
	s.rows[s.row] = line
}

func (s *ansiScreen) applyEraseChars(n int) {
	if n <= 0 {
		return
	}
	s.ensureRowCol()
	line := s.rows[s.row]
	for i := 0; i < n; i++ {
		pos := s.col + i
		if pos >= len(line) {
			break
		}
		line[pos] = ' '
	}
	s.rows[s.row] = line
}

func (s *ansiScreen) lines() []string {
	if len(s.rows) == 0 {
		return nil
	}

	keys := make([]int, 0, len(s.rows))
	for row := range s.rows {
		keys = append(keys, row)
	}
	sort.Ints(keys)

	out := make([]string, 0, len(keys))
	for _, row := range keys {
		line := strings.TrimRight(string(s.rows[row]), " ")
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}

	return out
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
	s := newANSIScreen()
	i := 0
	for i < len(input) {
		ch := input[i]

		switch ch {
		case '\n':
			s.row++
			s.col = 0
			i++
		case '\r':
			s.col = 0
			i++
		case '\t':
			for t := 0; t < 4; t++ {
				s.writeRune(' ')
			}
			i++
		case '\b':
			if s.col > 0 {
				s.col--
			}
			i++
		case 0x1b:
			consumed := consumeANSIEscape(input[i:], s)
			if consumed <= 0 {
				i++
				continue
			}
			i += consumed
		default:
			if !utf8.FullRuneInString(input[i:]) {
				i++
				continue
			}
			r, size := utf8.DecodeRuneInString(input[i:])
			i += size
			if r == utf8.RuneError && size == 1 {
				continue
			}
			if r >= 0x20 {
				s.writeRune(r)
			}
		}
	}

	return s.lines()
}

func consumeANSIEscape(input string, s *ansiScreen) int {
	if len(input) < 2 {
		return 1
	}

	next := input[1]
	switch next {
	case ']':
		// OSC: ESC ] ... BEL or ESC \
		i := 2
		for i < len(input) {
			if input[i] == '\a' {
				return i + 1
			}
			if input[i] == 0x1b && i+1 < len(input) && input[i+1] == '\\' {
				return i + 2
			}
			i++
		}
		return len(input)
	case '[':
		i := 2
		for i < len(input) {
			b := input[i]
			if b >= '@' && b <= '~' {
				params := input[2:i]
				s.applyCSI(params, b)
				return i + 1
			}
			i++
		}
		return len(input)
	default:
		if next == 'c' {
			// RIS - full terminal reset.
			s.reset()
		}
		return 2
	}
}

func hasPrivateMode(params string, mode int) bool {
	if params == "" {
		return false
	}
	needle := strconv.Itoa(mode)
	for _, part := range strings.Split(params, ";") {
		if strings.TrimPrefix(strings.TrimSpace(part), "?") == needle {
			return true
		}
	}
	return false
}

func parseCursorPos(params string) (int, int) {
	row := 1
	col := 1
	if params == "" {
		return row, col
	}
	parts := strings.Split(params, ";")
	if len(parts) >= 1 && strings.TrimPrefix(parts[0], "?") != "" {
		if parsed, err := strconv.Atoi(strings.TrimPrefix(parts[0], "?")); err == nil && parsed > 0 {
			row = parsed
		}
	}
	if len(parts) >= 2 && strings.TrimPrefix(parts[1], "?") != "" {
		if parsed, err := strconv.Atoi(strings.TrimPrefix(parts[1], "?")); err == nil && parsed > 0 {
			col = parsed
		}
	}
	return row, col
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
