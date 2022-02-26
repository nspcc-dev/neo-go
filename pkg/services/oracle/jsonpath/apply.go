package jsonpath

import (
	"fmt"

	json "github.com/nspcc-dev/go-ordered-json"
)

// apply filters value according to filter. The processing is done in DFS fashion,
// building resulting slice and it's JSON representation in parallel.
func (p *pathParser) apply(filter []node, value interface{}) ([]interface{}, bool) {
	if len(filter) == 0 {
		err := p.enc.Encode(value)
		if err != nil {
			return nil, false
		}
		p.buf.Bytes()
		if p.maxSize < p.buf.Len() {
			fmt.Println(p.buf.String())
			fmt.Println(p.buf.Len())
			return nil, false
		}
		return []interface{}{value}, true
	}

	switch filter[0].typ {
	case nodeAny:
		return p.descend(filter[1:], value)
	case nodeIndex:
		switch v := filter[0].value.(type) {
		case int:
			return p.descendByIndex(filter[1:], value, v)
		case string:
			return p.descendByIdent(filter[1:], value, v)
		default:
			panic("BUG: invalid value type")
		}
	case nodeIndexRecursive:
		name := filter[0].value.(string)
		objs := []interface{}{value}

		var values []interface{}
		for len(objs) > 0 {
			for i := range objs {
				newObjs, _ := p.descendByIdentAux(filter[1:], objs[i], false, name)
				values = append(values, newObjs...)
			}

			objs = p.flatten(objs)
		}
		return values, true
	case nodeUnion:
		switch v := filter[0].value.(type) {
		case []int:
			return p.descendByIndex(filter[1:], value, v...)
		case []string:
			return p.descendByIdent(filter[1:], value, v...)
		default:
			panic("BUG: unexpected union node type")
		}
	case nodeSlice:
		rng := filter[0].value.([2]int)
		return p.descendByRange(filter[1:], value, rng[0], rng[1])
	}
	return nil, true
}

func (p *pathParser) flatten(objs []interface{}) []interface{} {
	var values []interface{}
	for i := range objs {
		switch obj := objs[i].(type) {
		case []interface{}:
			values = append(values, obj...)
		case json.OrderedObject:
			for i := range obj {
				values = append(values, obj[i].Value)
			}
		}
	}
	return values
}

// descend descends 1 level down.
// It flattens arrays and returns map values for maps.
func (p *pathParser) descend(fs []node, obj interface{}) ([]interface{}, bool) {
	if p.depth <= 0 {
		return nil, false
	}
	p.depth--
	defer func() { p.depth++ }()

	var values []interface{}
	switch obj := obj.(type) {
	case []interface{}:
		for i := range obj {
			res, ok := p.apply(fs, obj[i])
			if !ok {
				return nil, false
			}
			values = append(values, res...)
		}
	case json.OrderedObject:
		for i := range obj {
			res, ok := p.apply(fs, obj[i].Value)
			if !ok {
				return nil, false
			}
			values = append(values, res...)
		}
	}
	return values, true
}

// descendByIdent performs map's field access by name.
func (p *pathParser) descendByIdent(fs []node, obj interface{}, names ...string) ([]interface{}, bool) {
	return p.descendByIdentAux(fs, obj, true, names...)
}

func (p *pathParser) descendByIdentAux(fs []node, obj interface{}, checkDepth bool, names ...string) ([]interface{}, bool) {
	if checkDepth {
		if p.depth <= 0 {
			return nil, false
		}
		p.depth--
		defer func() { p.depth++ }()
	}

	jmap, ok := obj.(json.OrderedObject)
	if !ok {
		return nil, true
	}

	var values []interface{}
	for j := range names {
		for k := range jmap {
			if jmap[k].Key == names[j] {
				res, ok := p.apply(fs, jmap[k].Value)
				if !ok {
					return nil, false
				}
				values = append(values, res...)
				break
			}
		}
	}
	return values, true
}

// descendByIndex performs array access by index.
func (p *pathParser) descendByIndex(fs []node, obj interface{}, indices ...int) ([]interface{}, bool) {
	if p.depth <= 0 {
		return nil, false
	}
	p.depth--
	defer func() { p.depth++ }()

	var values []interface{}
	arr, ok := obj.([]interface{})
	if !ok {
		return nil, true
	}

	for _, j := range indices {
		if j < 0 {
			j += len(arr)
		}
		if 0 <= j && j < len(arr) {
			res, ok := p.apply(fs, arr[j])
			if !ok {
				return nil, false
			}
			values = append(values, res...)
		}
	}

	return values, true
}

// descendByRange is similar to descend but skips maps and returns sub-slices for arrays.
func (p *pathParser) descendByRange(fs []node, obj interface{}, start, end int) ([]interface{}, bool) {
	if p.depth <= 0 {
		return nil, false
	}
	p.depth--

	var values []interface{}
	arr, ok := obj.([]interface{})
	if !ok {
		return nil, true
	}

	subStart := start
	if subStart < 0 {
		subStart += len(arr)
	}

	subEnd := end
	if subEnd <= 0 {
		subEnd += len(arr)
	}

	if subEnd > len(arr) {
		subEnd = len(arr)
	}

	if subEnd <= subStart {
		return nil, true
	}
	for j := subStart; j < subEnd; j++ {
		res, ok := p.apply(fs, arr[j])
		if !ok {
			return nil, false
		}
		values = append(values, res...)
	}

	return values, true
}
