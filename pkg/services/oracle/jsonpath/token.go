package jsonpath

func (p *pathParser) nextToken() (pathTokenType, string) {
	var (
		typ     pathTokenType
		value   string
		ok      = true
		numRead = 1
	)

	if p.i >= len(p.s) {
		return pathInvalid, ""
	}

	switch c := p.s[p.i]; c {
	case '$':
		typ = pathRoot
	case '.':
		typ = pathDot
	case '[':
		typ = pathLeftBracket
	case ']':
		typ = pathRightBracket
	case '*':
		typ = pathAsterisk
	case ',':
		typ = pathComma
	case ':':
		typ = pathColon
	case '\'':
		typ = pathString
		value, numRead, ok = p.parseString()
	default:
		switch {
		case c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z'):
			typ = pathIdentifier
			value, numRead, ok = p.parseIdent()
		case c == '-' || ('0' <= c && c <= '9'):
			typ = pathNumber
			value, numRead, ok = p.parseNumber()
		default:
			return pathInvalid, ""
		}
	}

	if !ok {
		return pathInvalid, ""
	}

	p.i += numRead
	return typ, value
}

// parseString parses JSON string surrounded by single quotes.
// It returns number of characters were consumed and true on success.
func (p *pathParser) parseString() (string, int, bool) {
	var end int
	for end = p.i + 1; end < len(p.s); end++ {
		if p.s[end] == '\'' {
			return p.s[p.i : end+1], end + 1 - p.i, true
		}
	}

	return "", 0, false
}

// parseIdent parses alphanumeric identifier.
// It returns number of characters were consumed and true on success.
func (p *pathParser) parseIdent() (string, int, bool) {
	var end int
	for end = p.i + 1; end < len(p.s); end++ {
		c := p.s[end]
		if c != '_' && !('a' <= c && c <= 'z') &&
			!('A' <= c && c <= 'Z') && !('0' <= c && c <= '9') {
			break
		}
	}

	return p.s[p.i:end], end - p.i, true
}

// parseNumber parses integer number.
// Only string representation is returned, size-checking is done on the first use.
// It also returns number of characters were consumed and true on success.
func (p *pathParser) parseNumber() (string, int, bool) {
	var end int
	for end = p.i + 1; end < len(p.s); end++ {
		c := p.s[end]
		if c < '0' || '9' < c {
			break
		}
	}

	return p.s[p.i:end], end - p.i, true
}
