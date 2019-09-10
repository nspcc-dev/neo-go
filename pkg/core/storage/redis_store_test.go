package storage

import (
	"reflect"
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/stretchr/testify/assert"
)

func TestNewRedisBatch(t *testing.T) {
	want := &RedisBatch{mem: map[string]string{}}
	if got := NewRedisBatch(); !reflect.DeepEqual(got, want) {
		t.Errorf("NewRedisBatch() = %v, want %v", got, want)
	}
}

func TestNewRedisStore(t *testing.T) {
	redisMock, redisStore := prepareRedisMock(t)
	key := []byte("testKey")
	value := []byte("testValue")
	err := redisStore.Put(key, value)
	assert.Nil(t, err, "NewRedisStore Put error")

	result, err := redisStore.Get(key)
	assert.Nil(t, err, "NewRedisStore Get error")

	assert.Equal(t, value, result)
	redisMock.Close()
}

func TestRedisBatch_Len(t *testing.T) {
	want := len(map[string]string{})
	b := &RedisBatch{
		mem: map[string]string{},
	}
	assert.Equal(t, len(b.mem), want)
}

func TestRedisBatch_Put(t *testing.T) {
	type args struct {
		k []byte
		v []byte
	}
	tests := []struct {
		name string
		args args
		want *RedisBatch
	}{
		{"TestRedisBatch_Put_Strings",
			args{
				k: []byte("foo"),
				v: []byte("bar"),
			},
			&RedisBatch{mem: map[string]string{"foo": "bar"}},
		},
		{"TestRedisBatch_Put_Numbers",
			args{
				k: []byte("123"),
				v: []byte("456"),
			},
			&RedisBatch{mem: map[string]string{"123": "456"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := &RedisBatch{mem: map[string]string{}}
			actual.Put(tt.args.k, tt.args.v)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func TestRedisStore_Batch(t *testing.T) {
	want := &RedisBatch{mem: map[string]string{}}
	actual := NewRedisBatch()
	assert.Equal(t, want, actual)
}

func TestRedisStore_GetAndPut(t *testing.T) {
	prepareRedisMock(t)
	type args struct {
		k       []byte
		v       []byte
		kToLook []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{"TestRedisStore_Get_Strings",
			args{
				k:       []byte("foo"),
				v:       []byte("bar"),
				kToLook: []byte("foo"),
			},
			[]byte("bar"),
			false,
		},
		{"TestRedisStore_Get_Negative_Strings",
			args{
				k:       []byte("foo"),
				v:       []byte("bar"),
				kToLook: []byte("wrong"),
			},
			[]byte(nil),
			true,
		},
	}
	redisMock, redisStore := prepareRedisMock(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := redisStore.Put(tt.args.k, tt.args.v)
			assert.Nil(t, err, "Got error while Put operation processing")
			got, err := redisStore.Get(tt.args.kToLook)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
			redisMock.FlushDB()
		})
	}
	redisMock.Close()
}

func TestRedisStore_PutBatch(t *testing.T) {
	batch := &RedisBatch{mem: map[string]string{"foo1": "bar1"}}
	mock, redisStore := prepareRedisMock(t)
	err := redisStore.PutBatch(batch)
	assert.Nil(t, err, "Error while PutBatch")
	result, err := redisStore.Get([]byte("foo1"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("bar1"), result)
	mock.Close()
}

func TestRedisStore_Seek(t *testing.T) {
	mock, redisStore := prepareRedisMock(t)
	redisStore.Seek([]byte("foo"), func(k, v []byte) {
		assert.Equal(t, []byte("bar"), v)
	})
	mock.Close()
}

func prepareRedisMock(t *testing.T) (*miniredis.Miniredis, *RedisStore) {
	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Errorf("MiniRedis mock creation error = %v", err)
	}
	_ = miniRedis.Set("foo", "bar")

	dbConfig := DBConfiguration{
		Type: "redisDB",
		RedisDBOptions: RedisDBOptions{
			Addr:     miniRedis.Addr(),
			Password: "",
			DB:       0,
		},
	}
	newRedisStore, err := NewRedisStore(dbConfig.RedisDBOptions)
	if err != nil {
		t.Errorf("NewRedisStore() error = %v", err)
		return nil, nil
	}
	return miniRedis, newRedisStore
}
