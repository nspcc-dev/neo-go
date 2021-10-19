package storage

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// MemCachedStore is a wrapper around persistent store that caches all changes
// being made for them to be later flushed in one batch.
type MemCachedStore struct {
	MemoryStore

	// plock protects Persist from double entrance.
	plock sync.Mutex
	// Persistent Store.
	ps Store
}

type (
	// KeyValue represents key-value pair.
	KeyValue struct {
		Key   []byte
		Value []byte
	}

	// KeyValueExists represents key-value pair with indicator whether the item
	// exists in the persistent storage.
	KeyValueExists struct {
		KeyValue

		Exists bool
	}

	// MemBatch represents a changeset to be persisted.
	MemBatch struct {
		Put     []KeyValueExists
		Deleted []KeyValueExists
	}
)

// NewMemCachedStore creates a new MemCachedStore object.
func NewMemCachedStore(lower Store) *MemCachedStore {
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		ps:          lower,
	}
}

// Get implements the Store interface.
func (s *MemCachedStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	k := string(key)
	if val, ok := s.mem[k]; ok {
		return val, nil
	}
	if _, ok := s.del[k]; ok {
		return nil, ErrKeyNotFound
	}
	return s.ps.Get(key)
}

// GetBatch returns currently accumulated changeset.
func (s *MemCachedStore) GetBatch() *MemBatch {
	s.mut.RLock()
	defer s.mut.RUnlock()

	var b MemBatch

	b.Put = make([]KeyValueExists, 0, len(s.mem))
	for k, v := range s.mem {
		key := []byte(k)
		_, err := s.ps.Get(key)
		b.Put = append(b.Put, KeyValueExists{KeyValue: KeyValue{Key: key, Value: v}, Exists: err == nil})
	}

	b.Deleted = make([]KeyValueExists, 0, len(s.del))
	for k := range s.del {
		key := []byte(k)
		_, err := s.ps.Get(key)
		b.Deleted = append(b.Deleted, KeyValueExists{KeyValue: KeyValue{Key: key}, Exists: err == nil})
	}

	return &b
}

// Seek implements the Store interface.
func (s *MemCachedStore) Seek(key []byte, f func(k, v []byte)) {
	s.seek(context.Background(), key, false, f)
}

// SeekAsync returns non-buffered channel with matching KeyValue pairs. Key and
// value slices may not be copied and may be modified. SeekAsync can guarantee
// that key-value items are sorted by key in ascending way.
func (s *MemCachedStore) SeekAsync(ctx context.Context, key []byte, cutPrefix bool) chan KeyValue {
	res := make(chan KeyValue)
	go func() {
		s.seek(ctx, key, cutPrefix, func(k, v []byte) {
			res <- KeyValue{
				Key:   k,
				Value: v,
			}
		})
		close(res)
	}()

	return res
}

func (s *MemCachedStore) seek(ctx context.Context, key []byte, cutPrefix bool, f func(k, v []byte)) {
	// Create memory store `mem` and `del` snapshot not to hold the lock.
	var memRes []KeyValueExists
	sk := string(key)
	s.mut.RLock()
	for k, v := range s.MemoryStore.mem {
		if strings.HasPrefix(k, sk) {
			memRes = append(memRes, KeyValueExists{
				KeyValue: KeyValue{
					Key:   []byte(k),
					Value: v,
				},
				Exists: true,
			})
		}
	}
	for k := range s.MemoryStore.del {
		if strings.HasPrefix(k, sk) {
			memRes = append(memRes, KeyValueExists{
				KeyValue: KeyValue{
					Key: []byte(k),
				},
			})
		}
	}
	ps := s.ps
	s.mut.RUnlock()
	// Sort memRes items for further comparison with ps items.
	sort.Slice(memRes, func(i, j int) bool {
		return bytes.Compare(memRes[i].Key, memRes[j].Key) < 0
	})

	var (
		done    bool
		iMem    int
		kvMem   KeyValueExists
		haveMem bool
	)
	if iMem < len(memRes) {
		kvMem = memRes[iMem]
		haveMem = true
		iMem++
	}
	// Merge results of seek operations in ascending order.
	ps.Seek(key, func(k, v []byte) {
		if done {
			return
		}
		kvPs := KeyValue{
			Key:   slice.Copy(k),
			Value: slice.Copy(v),
		}
	loop:
		for {
			select {
			case <-ctx.Done():
				done = true
				break loop
			default:
				var isMem = haveMem && (bytes.Compare(kvMem.Key, kvPs.Key) < 0)
				if isMem {
					if kvMem.Exists {
						if cutPrefix {
							kvMem.Key = kvMem.Key[len(key):]
						}
						f(kvMem.Key, kvMem.Value)
					}
					if iMem < len(memRes) {
						kvMem = memRes[iMem]
						haveMem = true
						iMem++
					} else {
						haveMem = false
					}
				} else {
					if !bytes.Equal(kvMem.Key, kvPs.Key) {
						if cutPrefix {
							kvPs.Key = kvPs.Key[len(key):]
						}
						f(kvPs.Key, kvPs.Value)
					}
					break loop
				}
			}
		}
	})
	if !done && haveMem {
	loop:
		for i := iMem - 1; i < len(memRes); i++ {
			select {
			case <-ctx.Done():
				break loop
			default:
				kvMem = memRes[i]
				if kvMem.Exists {
					if cutPrefix {
						kvMem.Key = kvMem.Key[len(key):]
					}
					f(kvMem.Key, kvMem.Value)
				}
			}
		}
	}
}

// Persist flushes all the MemoryStore contents into the (supposedly) persistent
// store ps.
func (s *MemCachedStore) Persist() (int, error) {
	var err error
	var keys, dkeys int

	s.plock.Lock()
	defer s.plock.Unlock()
	s.mut.Lock()

	keys = len(s.mem)
	dkeys = len(s.del)
	if keys == 0 && dkeys == 0 {
		s.mut.Unlock()
		return 0, nil
	}

	// tempstore technically copies current s in lower layer while real s
	// starts using fresh new maps. This tempstore is only known here and
	// nothing ever changes it, therefore accesses to it (reads) can go
	// unprotected while writes are handled by s proper.
	var tempstore = &MemCachedStore{MemoryStore: MemoryStore{mem: s.mem, del: s.del}, ps: s.ps}
	s.ps = tempstore
	s.mem = make(map[string][]byte)
	s.del = make(map[string]bool)
	s.mut.Unlock()

	err = tempstore.ps.PutChangeSet(tempstore.mem, tempstore.del)

	s.mut.Lock()
	if err == nil {
		// tempstore.mem and tempstore.del are completely flushed now
		// to tempstore.ps, so all KV pairs are the same and this
		// substitution has no visible effects.
		s.ps = tempstore.ps
	} else {
		// We're toast. We'll try to still keep proper state, but OOM
		// killer will get to us eventually.
		for k := range s.mem {
			tempstore.put(k, s.mem[k])
		}
		for k := range s.del {
			tempstore.drop(k)
		}
		s.ps = tempstore.ps
		s.mem = tempstore.mem
		s.del = tempstore.del
	}
	s.mut.Unlock()
	return keys, err
}

// Close implements Store interface, clears up memory and closes the lower layer
// Store.
func (s *MemCachedStore) Close() error {
	// It's always successful.
	_ = s.MemoryStore.Close()
	return s.ps.Close()
}
