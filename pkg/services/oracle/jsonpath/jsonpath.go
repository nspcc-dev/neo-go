package jsonpath

import (
	"strconv"
	"strings"

	json "github.com/nspcc-dev/go-ordered-json"
)

type (
	// pathTokenType represents a single JSONPath token.
	pathTokenType byte

	// pathParser combines a JSONPath and a position to start parsing from.
	pathParser struct {
		s     string
		i     int
		depth int
	}
)

const (
	pathInvalid pathTokenType = iota
	pathRoot
	pathDot
	pathLeftBracket
	pathRightBracket
	pathAsterisk
	pathComma
	pathColon
	pathIdentifier
	pathString
	pathNumber
)

const (
	maxNestingDepth = 6
	maxObjects      = 1024
)

// Get returns substructures of value selected by path.
// The result is always non-nil unless the path is invalid.
func Get(path string, value any) ([]any, bool) {
	if path == "" {
		return []any{value}, true
	}

	p := pathParser{
		depth: maxNestingDepth,
		s:     path,
	}

	typ, _ := p.nextToken()
	if typ != pathRoot {
		return nil, false
	}

	objs := []any{value}
	for p.i < len(p.s) {
		var ok bool

		switch typ, _ := p.nextToken(); typ {
		case pathDot:
			objs, ok = p.processDot(objs)
		case pathLeftBracket:
			objs, ok = p.processLeftBracket(objs)
		default:
		}

		if !ok || maxObjects < len(objs) {
			return nil, false
		}
	}

	if objs == nil {
		objs = []any{}
	}
	return objs, true
}

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

// parseString parses a JSON string surrounded by single quotes.
// It returns the number of characters consumed and true on success.
func (p *pathParser) parseString() (string, int, bool) {
	var end int
	for end = p.i + 1; end < len(p.s); end++ {
		if p.s[end] == '\'' {
			return p.s[p.i : end+1], end + 1 - p.i, true
		}
	}

	return "", 0, false
}

// parseIdent parses an alphanumeric identifier.
// It returns the number of characters consumed and true on success.
func (p *pathParser) parseIdent() (string, int, bool) {
	var end int
	for end = p.i + 1; end < len(p.s); end++ {
		c := p.s[end]
		if c != '_' && (c < 'a' || c > 'z') &&
			(c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			break
		}
	}

	return p.s[p.i:end], end - p.i, true
}

// parseNumber parses an integer number.
// Only string representation is returned, size-checking is done on the first use.
// It also returns the number of characters consumed and true on success.
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

// processDot handles `.` operator.
// It either descends 1 level down or performs recursive descent.
func (p *pathParser) processDot(objs []any) ([]any, bool) {
	typ, value := p.nextToken()
	switch typ {
	case pathAsterisk:
		return p.descend(objs)
	case pathDot:
		return p.descendRecursive(objs)
	case pathIdentifier:
		return p.descendByIdent(objs, value)
	default:
		return nil, false
	}
}

// descend descends 1 level down.
// It flattens arrays and returns map values for maps.
func (p *pathParser) descend(objs []any) ([]any, bool) {
	if p.depth <= 0 {
		return nil, false
	}
	p.depth--

	var values []any
	for i := range objs {
		switch obj := objs[i].(type) {
		case []any:
			if maxObjects < len(values)+len(obj) {
				return nil, false
			}
			values = append(values, obj...)
		case json.OrderedObject:
			if maxObjects < len(values)+len(obj) {
				return nil, false
			}
			for i := range obj {
				values = append(values, obj[i].Value)
			}
		}
	}

	return values, true
}

// descendRecursive performs recursive descent.
func (p *pathParser) descendRecursive(objs []any) ([]any, bool) {
	typ, val := p.nextToken()
	if typ != pathIdentifier {
		return nil, false
	}

	var values []any

	for len(objs) > 0 {
		newObjs, _ := p.descendByIdentAux(objs, false, val)
		if maxObjects < len(values)+len(newObjs) {
			return nil, false
		}
		values = append(values, newObjs...)
		objs, _ = p.descend(objs)
	}

	return values, true
}

// descendByIdent performs map's field access by name.
func (p *pathParser) descendByIdent(objs []any, names ...string) ([]any, bool) {
	return p.descendByIdentAux(objs, true, names...)
}

func (p *pathParser) descendByIdentAux(objs []any, checkDepth bool, names ...string) ([]any, bool) {
	if checkDepth {
		if p.depth <= 0 {
			return nil, false
		}
		p.depth--
	}

	var values []any
	for i := range objs {
		obj, ok := objs[i].(json.OrderedObject)
		if !ok {
			continue
		}

		for j := range names {
			for k := range obj {
				if obj[k].Key == names[j] {
					if maxObjects < len(values)+1 {
						return nil, false
					}
					values = append(values, obj[k].Value)
					break
				}
			}
		}
	}
	return values, true
}

// descendByIndex performs array access by index.
func (p *pathParser) descendByIndex(objs []any, indices ...int) ([]any, bool) {
	if p.depth <= 0 {
		return nil, false
	}
	p.depth--

	var values []any
	for i := range objs {
		obj, ok := objs[i].([]any)
		if !ok {
			continue
		}

		for _, j := range indices {
			if j < 0 {
				j += len(obj)
			}
			if 0 <= j && j < len(obj) {
				if maxObjects < len(values)+1 {
					return nil, false
				}
				values = append(values, obj[j])
			}
		}
	}

	return values, true
}

// processLeftBracket processes index expressions which can be either
// array/map access, array sub-slice or union of indices.
func (p *pathParser) processLeftBracket(objs []any) ([]any, bool) {
	typ, value := p.nextToken()
	switch typ {
	case pathAsterisk:
		typ, _ := p.nextToken()
		if typ != pathRightBracket {
			return nil, false
		}

		return p.descend(objs)
	case pathColon:
		return p.processSlice(objs, 0)
	case pathNumber:
		subTyp, _ := p.nextToken()
		switch subTyp {
		case pathColon:
			index, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return nil, false
			}

			return p.processSlice(objs, int(index))
		case pathComma:
			return p.processUnion(objs, pathNumber, value)
		case pathRightBracket:
			index, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return nil, false
			}

			return p.descendByIndex(objs, int(index))
		default:
			return nil, false
		}
	case pathString:
		subTyp, _ := p.nextToken()
		switch subTyp {
		case pathComma:
			return p.processUnion(objs, pathString, value)
		case pathRightBracket:
			s := strings.Trim(value, "'")
			err := json.Unmarshal([]byte(`"`+s+`"`), &s)
			if err != nil {
				return nil, false
			}
			return p.descendByIdent(objs, s)
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

// processUnion processes union of multiple indices.
// firstTyp is assumed to be either pathNumber or pathString.
func (p *pathParser) processUnion(objs []any, firstTyp pathTokenType, firstVal string) ([]any, bool) {
	items := []string{firstVal}
	for {
		typ, val := p.nextToken()
		if typ != firstTyp {
			return nil, false
		}

		items = append(items, val)
		typ, _ = p.nextToken()
		if typ == pathRightBracket {
			break
		} else if typ != pathComma {
			return nil, false
		}
	}

	switch firstTyp {
	case pathNumber:
		values := make([]int, len(items))
		for i := range items {
			index, err := strconv.ParseInt(items[i], 10, 32)
			if err != nil {
				return nil, false
			}
			values[i] = int(index)
		}
		return p.descendByIndex(objs, values...)
	case pathString:
		for i := range items {
			s := strings.Trim(items[i], "'")
			err := json.Unmarshal([]byte(`"`+s+`"`), &items[i])
			if err != nil {
				return nil, false
			}
		}
		return p.descendByIdent(objs, items...)
	default:
		panic("token in union must be either number or string")
	}
}

// processSlice processes a slice with the specified start index.
func (p *pathParser) processSlice(objs []any, start int) ([]any, bool) {
	typ, val := p.nextToken()
	switch typ {
	case pathNumber:
		typ, _ := p.nextToken()
		if typ != pathRightBracket {
			return nil, false
		}

		index, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return nil, false
		}

		return p.descendByRange(objs, start, int(index))
	case pathRightBracket:
		return p.descendByRange(objs, start, 0)
	default:
		return nil, false
	}
}

// descendByRange is similar to descend but skips maps and returns sub-slices for arrays.
func (p *pathParser) descendByRange(objs []any, start, end int) ([]any, bool) {
	if p.depth <= 0 {
		return nil, false
	}
	p.depth--

	var values []any
	for i := range objs {
		arr, ok := objs[i].([]any)
		if !ok {
			continue
		}

		subStart := start
		if subStart < 0 {
			subStart += len(arr)
		}

		subEnd := end
		if subEnd <= 0 {
			subEnd += len(arr)
		}

		subEnd = min(subEnd, len(arr))

		if subEnd <= subStart {
			continue
		}
		if maxObjects < len(values)+subEnd-subStart {
			return nil, false
		}
		values = append(values, arr[subStart:subEnd]...)
	}

	return values, true
}
