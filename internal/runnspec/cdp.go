package runnspec

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/teyamagu/runnlint/internal/message"
)

var (
	cdpArgRe   = regexp.MustCompile(`\{CDPArgTypeArg,\s*"([^"]+)"`)
	cdpAliasRe = regexp.MustCompile(`Aliases:\s*\[\]string\{([^}]*)\}`)
	cdpNameRe  = regexp.MustCompile(`"([^"]+)"`)
)

// ParseCDPSource reads CDPFnMap names, aliases, and input arguments from runn's cdpfn.go.
func ParseCDPSource(src string) (map[string][]string, error) {
	const marker = "var CDPFnMap = map[string]CDPFn{"
	at := strings.Index(src, marker)
	if at < 0 {
		return nil, message.New(message.CDPMapMissing)
	}
	open := at + len(marker) - 1
	body, err := braceBody(src, open)
	if err != nil {
		return nil, err
	}
	entries, err := topLevelObjects(body)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, message.New(message.CDPMapEmpty)
	}
	out := map[string][]string{}
	for name, entry := range entries {
		args := inputArgs(entry)
		out[name] = args
		for _, alias := range aliases(entry) {
			out[alias] = args
		}
	}
	return out, nil
}

func inputArgs(entry string) []string {
	var args []string
	for _, match := range cdpArgRe.FindAllStringSubmatch(entry, -1) {
		args = append(args, match[1])
	}
	return args
}

func aliases(entry string) []string {
	var names []string
	for _, match := range cdpAliasRe.FindAllStringSubmatch(entry, -1) {
		for _, name := range cdpNameRe.FindAllStringSubmatch(match[1], -1) {
			names = append(names, name[1])
		}
	}
	return names
}

func braceBody(src string, open int) (string, error) {
	end, err := matchingBrace(src, open)
	if err != nil {
		return "", err
	}
	return src[open+1 : end], nil
}

func matchingBrace(src string, open int) (int, error) {
	sc := scanner{src: src, i: open}
	if sc.peek() != '{' {
		return 0, message.New(message.CDPMapStart)
	}
	sc.next()
	depth := 1
	for sc.i < len(src) {
		if sc.skipSpaceComments() {
			continue
		}
		switch sc.peek() {
		case '"':
			if _, err := sc.readString(); err != nil {
				return 0, err
			}
		case '`':
			skipRaw(&sc)
		case '{':
			depth++
			sc.next()
		case '}':
			depth--
			if depth == 0 {
				return sc.i, nil
			}
			sc.next()
		default:
			sc.next()
		}
	}
	return 0, message.New(message.CDPMapClose)
}

// topLevelObjects returns depth-0 "name": { ... } entries from the body of a map literal.
func topLevelObjects(body string) (map[string]string, error) {
	sc := scanner{src: body}
	out := map[string]string{}
	for sc.i < len(sc.src) {
		if sc.skipSpaceComments() {
			continue
		}
		if sc.peek() != '"' {
			sc.next()
			continue
		}
		name, err := sc.readString()
		if err != nil {
			return nil, err
		}
		sc.skipSpaceComments()
		if sc.peek() != ':' {
			continue
		}
		sc.next()
		sc.skipSpaceComments()
		if sc.peek() != '{' {
			continue
		}
		end, err := matchingBrace(sc.src, sc.i)
		if err != nil {
			return nil, err
		}
		out[name] = sc.src[sc.i : end+1]
		sc.i = end + 1
	}
	return out, nil
}

type scanner struct {
	src string
	i   int
}

func (s *scanner) peek() byte {
	if s.i >= len(s.src) {
		return 0
	}
	return s.src[s.i]
}

func (s *scanner) next() {
	if s.i < len(s.src) {
		s.i++
	}
}

func (s *scanner) skipSpaceComments() bool {
	if s.i >= len(s.src) {
		return false
	}
	switch s.peek() {
	case ' ', '\t', '\n', '\r':
		s.next()
		return true
	case '/':
		if strings.HasPrefix(s.src[s.i:], "//") {
			for s.i < len(s.src) && s.peek() != '\n' {
				s.next()
			}
			return true
		}
		if strings.HasPrefix(s.src[s.i:], "/*") {
			s.i += 2
			for s.i+1 < len(s.src) && s.src[s.i:s.i+2] != "*/" {
				s.next()
			}
			if s.i+1 < len(s.src) {
				s.i += 2
			}
			return true
		}
	}
	return false
}

func skipRaw(s *scanner) {
	s.next()
	for s.i < len(s.src) && s.peek() != '`' {
		s.next()
	}
	s.next()
}

func (s *scanner) readString() (string, error) {
	if s.peek() != '"' {
		return "", message.New(message.NotString)
	}
	s.next()
	start := s.i
	for s.i < len(s.src) {
		switch s.peek() {
		case '\\':
			s.next()
			s.next()
		case '"':
			value := s.src[start:s.i]
			s.next()
			if !utf8.ValidString(value) {
				return "", message.New(message.BadString)
			}
			return value, nil
		default:
			s.next()
		}
	}
	return "", message.New(message.StringUnclosed)
}
