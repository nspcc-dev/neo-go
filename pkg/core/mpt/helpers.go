package mpt

// lcp returns longest common prefix of a and b.
// Note: it does no allocations.
func lcp(a, b []byte) []byte {
	if len(a) < len(b) {
		return lcp(b, a)
	}

	var i int
	for i = 0; i < len(b); i++ {
		if a[i] != b[i] {
			break
		}
	}

	return a[:i]
}

func lcpMany(kv []keyValue) []byte {
	if len(kv) == 1 {
		return kv[0].key
	}
	p := lcp(kv[0].key, kv[1].key)
	if len(p) == 0 {
		return p
	}
	for i := range kv[2:] {
		p = lcp(p, kv[2+i].key)
	}
	return p
}

// toNibbles mangles path by splitting every byte into 2 containing low- and high- 4-byte part.
func toNibbles(path []byte) []byte {
	result := make([]byte, len(path)*2)
	for i := range path {
		result[i*2] = path[i] >> 4
		result[i*2+1] = path[i] & 0x0F
	}
	return result
}
