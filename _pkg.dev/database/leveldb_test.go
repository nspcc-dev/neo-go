package database_test

import (
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/stretchr/testify/assert"
)

const path = "temp"

func cleanup(db *database.LDB) {
	db.Close()
	os.RemoveAll(database.DbDir)
}
func TestDBCreate(t *testing.T) {

	db, err := database.New(path)
	assert.Nil(t, err)

	assert.NotEqual(t, nil, db)
	cleanup(db)
}
func TestPutGet(t *testing.T) {

	db, err := database.New(path)
	assert.Nil(t, err)

	key := []byte("Hello")
	value := []byte("World")

	err = db.Put(key, value)
	assert.Equal(t, nil, err)

	res, err := db.Get(key)
	assert.Equal(t, nil, err)
	assert.Equal(t, value, res)
	cleanup(db)
}
func TestPutDelete(t *testing.T) {

	db, err := database.New(path)
	assert.Nil(t, err)

	key := []byte("Hello")
	value := []byte("World")

	err = db.Put(key, value)

	err = db.Delete(key)
	assert.Equal(t, nil, err)

	res, err := db.Get(key)

	assert.Equal(t, database.ErrNotFound, err)
	assert.Equal(t, res, []byte{})
	cleanup(db)
}

func TestHas(t *testing.T) {

	db, err := database.New(path)
	assert.Nil(t, err)

	res, err := db.Has([]byte("NotExist"))
	assert.Equal(t, res, false)
	assert.Equal(t, err, nil)

	key := []byte("Hello")
	value := []byte("World")

	err = db.Put(key, value)
	assert.Equal(t, nil, err)

	res, err = db.Has(key)
	assert.Equal(t, res, true)
	assert.Equal(t, err, nil)
	cleanup(db)

}
func TestDBClose(t *testing.T) {

	db, err := database.New(path)
	assert.Nil(t, err)

	err = db.Close()
	assert.Equal(t, nil, err)

	cleanup(db)
}
