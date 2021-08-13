package storage

import (
	"fmt"

	"github.com/go-redis/redis"
)

// RedisDBOptions configuration for RedisDB.
type RedisDBOptions struct {
	Addr     string `yaml:"Addr"`
	Password string `yaml:"Password"`
	DB       int    `yaml:"DB"`
}

// RedisStore holds the client and maybe later some more metadata.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore returns an new initialized - ready to use RedisStore object.
func NewRedisStore(cfg RedisDBOptions) (*RedisStore, error) {
	c := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if _, err := c.Ping().Result(); err != nil {
		return nil, err
	}
	return &RedisStore{client: c}, nil
}

// Batch implements the Store interface.
func (s *RedisStore) Batch() Batch {
	return newMemoryBatch()
}

// Get implements the Store interface.
func (s *RedisStore) Get(k []byte) ([]byte, error) {
	val, err := s.client.Get(string(k)).Result()
	if err != nil {
		if err == redis.Nil {
			err = ErrKeyNotFound
		}
		return nil, err
	}
	return []byte(val), nil
}

// Delete implements the Store interface.
func (s *RedisStore) Delete(k []byte) error {
	s.client.Del(string(k))
	return nil
}

// Put implements the Store interface.
func (s *RedisStore) Put(k, v []byte) error {
	s.client.Set(string(k), string(v), 0)
	return nil
}

// PutBatch implements the Store interface.
func (s *RedisStore) PutBatch(b Batch) error {
	memBatch := b.(*MemoryBatch)
	return s.PutChangeSet(memBatch.mem, memBatch.del)
}

// PutChangeSet implements the Store interface.
func (s *RedisStore) PutChangeSet(puts map[string][]byte, dels map[string]bool) error {
	pipe := s.client.Pipeline()
	for k, v := range puts {
		pipe.Set(k, v, 0)
	}
	for k := range dels {
		pipe.Del(k)
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

// Close implements the Store interface.
func (s *RedisStore) Close() error {
	return s.client.Close()
}
