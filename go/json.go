// Copyright (c) 2026 tabnas, MIT License

// Package json is a standard JSON parser (RFC 8259 / ECMA-404). It accepts
// exactly the JSON grammar — objects, arrays, strings, numbers, and the
// literals true, false and null — and nothing else. There is no extended
// grammar: no comments, no trailing commas, no unquoted keys, no
// single-quoted or multiline strings, no implicit objects or arrays.
// Anything encoding/json would reject, this parser rejects too.
package json

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Version is the current version of the module.
const Version = "1.0.0"

// Parse parses a JSON source string and returns the resulting value:
// nil, bool, float64, string, []any, or map[string]any. On any input that
// is not valid standard JSON it returns a *JsonError.
func Parse(src string) (any, error) {
	p := &parser{src: src, len: len(src)}
	value, err := p.parse()
	if err != nil {
		return nil, err
	}
	return value, nil
}

type parser struct {
	src string
	len int
	i   int
}

func (p *parser) parse() (any, error) {
	p.skipWs()
	if p.i >= p.len {
		return nil, p.fail("unexpected_eof", "unexpected end of input")
	}
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipWs()
	if p.i < p.len {
		return nil, p.fail("trailing_content", "unexpected character "+quoteByte(p.src[p.i]))
	}
	return value, nil
}

func (p *parser) parseValue() (any, error) {
	c := p.src[p.i]
	switch {
	case c == '{':
		return p.parseObject()
	case c == '[':
		return p.parseArray()
	case c == '"':
		return p.parseString()
	case c == 't':
		return p.parseLiteral("true", true)
	case c == 'f':
		return p.parseLiteral("false", false)
	case c == 'n':
		return p.parseLiteral("null", nil)
	case c == '-' || isDigit(c):
		return p.parseNumber()
	default:
		return nil, p.fail("unexpected_char", "unexpected character "+quoteByte(c))
	}
}

func (p *parser) parseObject() (any, error) {
	p.i++ // consume {
	obj := map[string]any{}
	p.skipWs()
	if p.i < p.len && p.src[p.i] == '}' {
		p.i++
		return obj, nil
	}
	for {
		p.skipWs()
		if p.i >= p.len {
			return nil, p.fail("unexpected_eof", "unexpected end of input in object")
		}
		if p.src[p.i] != '"' {
			return nil, p.fail("expected_key", "expected a string key")
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipWs()
		if p.i >= p.len || p.src[p.i] != ':' {
			return nil, p.fail("expected_colon", "expected ':' after object key")
		}
		p.i++ // consume :
		p.skipWs()
		if p.i >= p.len {
			return nil, p.fail("unexpected_eof", "unexpected end of input in object")
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		obj[key] = value
		p.skipWs()
		if p.i >= p.len {
			return nil, p.fail("unexpected_eof", "unexpected end of input in object")
		}
		switch p.src[p.i] {
		case ',':
			p.i++
			continue
		case '}':
			p.i++
			return obj, nil
		default:
			return nil, p.fail("expected_comma", "expected ',' or '}' in object")
		}
	}
}

func (p *parser) parseArray() (any, error) {
	p.i++ // consume [
	arr := []any{}
	p.skipWs()
	if p.i < p.len && p.src[p.i] == ']' {
		p.i++
		return arr, nil
	}
	for {
		p.skipWs()
		if p.i >= p.len {
			return nil, p.fail("unexpected_eof", "unexpected end of input in array")
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, value)
		p.skipWs()
		if p.i >= p.len {
			return nil, p.fail("unexpected_eof", "unexpected end of input in array")
		}
		switch p.src[p.i] {
		case ',':
			p.i++
			continue
		case ']':
			p.i++
			return arr, nil
		default:
			return nil, p.fail("expected_comma", "expected ',' or ']' in array")
		}
	}
}

func (p *parser) parseString() (string, error) {
	p.i++ // consume opening "
	var b strings.Builder
	start := p.i
	for {
		if p.i >= p.len {
			return "", p.fail("unterminated_string", "unterminated string")
		}
		c := p.src[p.i]
		switch {
		case c == '"':
			b.WriteString(p.src[start:p.i])
			p.i++
			return b.String(), nil
		case c == '\\':
			b.WriteString(p.src[start:p.i])
			p.i++
			s, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			b.WriteString(s)
			start = p.i
		case c < 0x20:
			return "", p.fail("invalid_string_char", "control character must be escaped in a string")
		default:
			p.i++
		}
	}
}

func (p *parser) parseEscape() (string, error) {
	if p.i >= p.len {
		return "", p.fail("unterminated_string", "unterminated string")
	}
	e := p.src[p.i]
	switch e {
	case '"':
		p.i++
		return "\"", nil
	case '\\':
		p.i++
		return "\\", nil
	case '/':
		p.i++
		return "/", nil
	case 'b':
		p.i++
		return "\b", nil
	case 'f':
		p.i++
		return "\f", nil
	case 'n':
		p.i++
		return "\n", nil
	case 'r':
		p.i++
		return "\r", nil
	case 't':
		p.i++
		return "\t", nil
	case 'u':
		return p.parseUnicodeEscape()
	default:
		return "", p.fail("invalid_escape", "invalid escape sequence \\"+string(e))
	}
}

func (p *parser) parseUnicodeEscape() (string, error) {
	// At call time, src[i] == 'u'.
	code, err := p.readHex4()
	if err != nil {
		return "", err
	}
	if code >= 0xd800 && code <= 0xdbff {
		// High surrogate — pair it with a following low surrogate escape.
		if p.i+1 < p.len && p.src[p.i] == '\\' && p.src[p.i+1] == 'u' {
			p.i++ // consume backslash, leaving src[i] == 'u'
			low, err := p.readHex4()
			if err != nil {
				return "", err
			}
			if low >= 0xdc00 && low <= 0xdfff {
				r := 0x10000 + (rune(code)-0xd800)*0x400 + (rune(low) - 0xdc00)
				return string(r), nil
			}
			return string(rune(0xfffd)) + encodeCodeUnit(low), nil
		}
		return string(rune(0xfffd)), nil
	}
	if code >= 0xdc00 && code <= 0xdfff {
		// Lone low surrogate.
		return string(rune(0xfffd)), nil
	}
	return string(rune(code)), nil
}

// readHex4 reads `uXXXX` starting at src[i] == 'u' and returns the value.
func (p *parser) readHex4() (int, error) {
	p.i++ // consume u
	if p.i+4 > p.len {
		return 0, p.fail("invalid_unicode", "incomplete \\u escape")
	}
	code := 0
	for k := 0; k < 4; k++ {
		h := p.src[p.i]
		var d int
		switch {
		case h >= '0' && h <= '9':
			d = int(h - '0')
		case h >= 'a' && h <= 'f':
			d = int(h-'a') + 10
		case h >= 'A' && h <= 'F':
			d = int(h-'A') + 10
		default:
			return 0, p.fail("invalid_unicode", "invalid hex digit "+quoteByte(h)+" in \\u escape")
		}
		code = code*16 + d
		p.i++
	}
	return code, nil
}

func (p *parser) parseNumber() (any, error) {
	start := p.i
	if p.src[p.i] == '-' {
		p.i++
	}
	if p.i >= p.len {
		return nil, p.fail("invalid_number", "invalid number: expected a digit")
	}
	// integer part
	if p.src[p.i] == '0' {
		p.i++ // a single leading zero
	} else if isDigit(p.src[p.i]) {
		for p.i < p.len && isDigit(p.src[p.i]) {
			p.i++
		}
	} else {
		return nil, p.fail("invalid_number", "invalid number: expected a digit")
	}
	// fraction
	if p.i < p.len && p.src[p.i] == '.' {
		p.i++
		if p.i >= p.len || !isDigit(p.src[p.i]) {
			return nil, p.fail("invalid_number", "invalid number: expected a digit after decimal point")
		}
		for p.i < p.len && isDigit(p.src[p.i]) {
			p.i++
		}
	}
	// exponent
	if p.i < p.len && (p.src[p.i] == 'e' || p.src[p.i] == 'E') {
		p.i++
		if p.i < p.len && (p.src[p.i] == '+' || p.src[p.i] == '-') {
			p.i++
		}
		if p.i >= p.len || !isDigit(p.src[p.i]) {
			return nil, p.fail("invalid_number", "invalid number: expected a digit in exponent")
		}
		for p.i < p.len && isDigit(p.src[p.i]) {
			p.i++
		}
	}
	f, err := strconv.ParseFloat(p.src[start:p.i], 64)
	if err != nil {
		return nil, p.fail("invalid_number", "invalid number: "+p.src[start:p.i])
	}
	return f, nil
}

func (p *parser) parseLiteral(word string, value any) (any, error) {
	if strings.HasPrefix(p.src[p.i:], word) {
		p.i += len(word)
		return value, nil
	}
	return nil, p.fail("invalid_literal", "expected "+strconv.Quote(word))
}

func (p *parser) skipWs() {
	for p.i < p.len {
		c := p.src[p.i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			p.i++
		} else {
			break
		}
	}
}

func (p *parser) fail(code, detail string) *JsonError {
	line, col := 1, 1
	for k := 0; k < p.i && k < p.len; k++ {
		if p.src[k] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &JsonError{Code: code, Detail: detail, Index: p.i, Line: line, Column: col}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func quoteByte(c byte) string { return strconv.Quote(string(c)) }

// encodeCodeUnit renders a single UTF-16 code unit as a UTF-8 string,
// substituting the replacement character for unpaired surrogates.
func encodeCodeUnit(code int) string {
	if code >= 0xd800 && code <= 0xdfff {
		return string(rune(0xfffd))
	}
	r := rune(code)
	if !utf8.ValidRune(r) {
		return string(rune(0xfffd))
	}
	return string(r)
}
