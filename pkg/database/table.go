package database

//Table is an abstract data structure built on top of a db
type Table struct {
	prefix []byte
	db     Database
}

//NewTable creates a new table on the given database
func NewTable(db Database, prefix []byte) *Table {
	return &Table{
		prefix,
		db,
	}
}

// Has implements the database interface
func (t *Table) Has(key []byte) (bool, error) {
	prefixedKey := append(t.prefix, key...)
	return t.db.Has(prefixedKey)
}

// Put implements the database interface
func (t *Table) Put(key []byte, value []byte) error {
	prefixedKey := append(t.prefix, key...)
	return t.db.Put(prefixedKey, value)
}

// Get implements the database interface
func (t *Table) Get(key []byte) ([]byte, error) {
	prefixedKey := append(t.prefix, key...)
	return t.db.Get(prefixedKey)
}

// Delete implements the database interface
func (t *Table) Delete(key []byte) error {
	prefixedKey := append(t.prefix, key...)
	return t.db.Delete(prefixedKey)
}

// Close implements the database interface
func (t *Table) Close() error {
	return nil
}

// Prefix implements the database interface
func (t *Table) Prefix(key []byte) ([][]byte, error) {
	prefixedKey := append(t.prefix, key...)
	return t.db.Prefix(prefixedKey)
}
