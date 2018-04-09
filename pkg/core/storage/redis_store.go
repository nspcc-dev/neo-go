package storage

import (
	"fmt"

	"github.com/go-redis/redis"
)

// RedisStore holds the client and maybe later some more metadata.
type RedisStore struct {
	client *redis.Client
}

// RedisBatch simple batch implementation to satisfy the Store interface.
type RedisBatch struct {
	mem map[string]string
}

// Len implements the Batch interface.
func (b *RedisBatch) Len() int {
	return len(b.mem)
}

// Put implements the Batch interface.
func (b *RedisBatch) Put(k, v []byte) {
	b.mem[string(k)] = string(v)
}

// NewRedisBatch returns a new ready to use RedisBatch.
func NewRedisBatch() *RedisBatch {
	return &RedisBatch{
		mem: make(map[string]string),
	}
}

// NewRedisStore returns an new initialized - ready to use RedisStore object
func NewRedisStore() (*RedisStore, error) {
	c := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if _, err := c.Ping().Result(); err != nil {
		return nil, err
	}
	return &RedisStore{
		client: c,
	}, nil
}

// Batch implements the Store interface.
func (s *RedisStore) Batch() Batch {
	return NewRedisBatch()
}

// Get implements the Store interface.
func (s *RedisStore) Get(k []byte) ([]byte, error) {
	val, err := s.client.Get(string(k)).Result()
	if err != nil {
		return nil, err
	}
	return []byte(val), nil
}

// Put implements the Store interface.
func (s *RedisStore) Put(k, v []byte) error {
	s.client.Set(string(k), string(v), 0)
	return nil
}

// PutBatch implements the Store interface.
func (s *RedisStore) PutBatch(b Batch) error {
	pipe := s.client.Pipeline()
	for k, v := range b.(*RedisBatch).mem {
		pipe.Set(k, v, 0)
	}
	_, err := pipe.Exec()
	return err
}

// Seek implements the Store interface.
func (s *RedisStore) Seek(k []byte, f func(k, v []byte)) {
	iter := s.client.Scan(0, fmt.Sprintf("%s*", k), 0).Iterator()
	for iter.Next() {
		key := iter.Val()
		val, _ := s.client.Get(key).Result()
		f([]byte(key), []byte(val))
	}
}
