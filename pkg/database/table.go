package database

//Table is an abstract datastructure built on
// top of a db
type Table struct {
	prefix []byte
	db     Database
}

func NewTable(db Database, prefix []byte) *Table {
	return &Table{
		prefix,
		db,
	}
}

func (t *Table) Has(key []byte) (bool, error) {
	key = append(t.prefix, key...)
	return t.db.Has(key)
}

func (t *Table) Put(key []byte, value []byte) error {
	key = append(t.prefix, key...)
	return t.db.Put(key, value)
}
func (t *Table) Get(key []byte) ([]byte, error) {
	key = append(t.prefix, key...)
	return t.db.Get(key)
}
func (t *Table) Delete(key []byte) error {
	key = append(t.prefix, key...)
	return t.db.Delete(key)
}
func (t *Table) Close() error {
	return nil
}
