package jsonpath

import (
	"bytes"
	"strconv"
	"strings"

	json "github.com/nspcc-dev/go-ordered-json"
)

type (
	// pathTokenType represents single JSONPath token.
	pathTokenType byte

	// pathParser combines JSONPath and a position to start parsing from.
	pathParser struct {
		s       string
		i       int
		depth   int
		maxSize int
		buf     *bytes.Buffer
		enc     *json.Encoder
	}

	nodeType byte

	node struct {
		typ   nodeType
		value interface{}
	}
)

const (
	nodeAny nodeType = iota
	nodeIndex
	nodeIndexRecursive
	nodeUnion
	nodeSlice
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

const maxNestingDepth = 6

// Get returns substructures of value selected by path.
// The result is always non-nil unless path is invalid.
func Get(path string, value interface{}, maxSize int) ([]interface{}, json.RawMessage, bool) {
	if path == "" {
		val := []interface{}{value}
		data, err := json.Marshal(val)
		return val, data, err == nil
	}

	buf := bytes.NewBuffer(nil)
	p := pathParser{
		depth:   maxNestingDepth,
		s:       path,
		maxSize: maxSize,
		buf:     buf,
		enc:     json.NewEncoder(buf),
	}

	typ, _ := p.nextToken()
	if typ != pathRoot {
		return nil, nil, false
	}

	var ns []node
	for p.i < len(p.s) {
		var ok bool
		var n node

		switch typ, _ := p.nextToken(); typ {
		case pathDot:
			n, ok = p.processDot()
		case pathLeftBracket:
			n, ok = p.processLeftBracket()
		}

		if !ok {
			return nil, nil, false
		}
		ns = append(ns, n)
	}

	objs, ok := p.apply(ns, value)
	if !ok {
		return nil, nil, false
	}

	if objs == nil {
		objs = []interface{}{}
	}
	return objs, p.buf.Bytes(), true
}

// processDot handles `.` operator.
// It either descends 1 level down or performs recursive descent.
func (p *pathParser) processDot() (node, bool) {
	typ, value := p.nextToken()
	switch typ {
	case pathAsterisk:
		return node{nodeAny, nil}, true
	case pathDot:
		return p.processDotRecursive()
	case pathIdentifier:
		return node{nodeIndex, value}, true
	default:
		return node{}, false
	}
}

// processDotRecursive performs recursive descent.
func (p *pathParser) processDotRecursive() (node, bool) {
	typ, val := p.nextToken()
	if typ != pathIdentifier {
		return node{}, false
	}
	return node{nodeIndexRecursive, val}, true
}

// processLeftBracket processes index expressions which can be either
// array/map access, array sub-slice or union of indices.
func (p *pathParser) processLeftBracket() (node, bool) {
	typ, value := p.nextToken()
	switch typ {
	case pathAsterisk:
		typ, _ := p.nextToken()
		if typ != pathRightBracket {
			return node{}, false
		}
		return node{nodeAny, nil}, true
	case pathColon:
		return p.processSlice(0)
	case pathNumber:
		subTyp, _ := p.nextToken()
		switch subTyp {
		case pathColon:
			index, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return node{}, false
			}

			return p.processSlice(int(index))
		case pathComma:
			return p.processUnion(pathNumber, value)
		case pathRightBracket:
			index, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return node{}, false
			}
			return node{nodeIndex, int(index)}, true
		default:
			return node{}, false
		}
	case pathString:
		subTyp, _ := p.nextToken()
		switch subTyp {
		case pathComma:
			return p.processUnion(pathString, value)
		case pathRightBracket:
			s := strings.Trim(value, "'")
			err := json.Unmarshal([]byte(`"`+s+`"`), &s)
			if err != nil {
				return node{}, false
			}
			return node{nodeIndex, s}, true
		default:
			return node{}, false
		}
	default:
		return node{}, false
	}
}

// processUnion processes union of multiple indices.
// firstTyp is assumed to be either pathNumber or pathString.
func (p *pathParser) processUnion(firstTyp pathTokenType, firstVal string) (node, bool) {
	items := []string{firstVal}
	for {
		typ, val := p.nextToken()
		if typ != firstTyp {
			return node{}, false
		}

		items = append(items, val)
		typ, _ = p.nextToken()
		if typ == pathRightBracket {
			break
		} else if typ != pathComma {
			return node{}, false
		}
	}

	switch firstTyp {
	case pathNumber:
		values := make([]int, len(items))
		for i := range items {
			index, err := strconv.ParseInt(items[i], 10, 32)
			if err != nil {
				return node{}, false
			}
			values[i] = int(index)
		}
		return node{nodeUnion, values}, true
	case pathString:
		for i := range items {
			s := strings.Trim(items[i], "'")
			err := json.Unmarshal([]byte(`"`+s+`"`), &items[i])
			if err != nil {
				return node{}, false
			}
		}
		return node{nodeUnion, items}, true
	default:
		panic("token in union must be either number or string")
	}
}

// processSlice processes slice with the specified start index.
func (p *pathParser) processSlice(start int) (node, bool) {
	typ, val := p.nextToken()
	switch typ {
	case pathNumber:
		typ, _ := p.nextToken()
		if typ != pathRightBracket {
			return node{}, false
		}

		index, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return node{}, false
		}
		return node{nodeSlice, [2]int{start, int(index)}}, true
	case pathRightBracket:
		return node{nodeSlice, [2]int{start, 0}}, true
	default:
		return node{}, false
	}
}
