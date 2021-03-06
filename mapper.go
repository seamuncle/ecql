package ecql

import (
	"reflect"
	"strings"
	"sync"
)

var (
	// TAG_COLUMNS is the tag used in the structs to set the column name for a field.
	// If a name is not set, the name would be the lowercase version of the field.
	// If you want to skip a field you can use `cql:"-"`
	TAG_COLUMN = "cql"

	// TAG_TABLE is the tag used in the structs to define the table for a type.
	// If the table is not set it defaults to the type name in lowercase.
	TAG_TABLE = "cqltable"

	// TAG_KEY defines the primary key for the table.
	// If the table uses a composite key you just need to define multiple columns
	// separated by a comma: `cqlkey:"id"` or `cqlkey:"partkey,id"`
	TAG_KEY = "cqlkey"
)

var registry = newSyncRegistry()

type syncRegistry struct {
	sync.RWMutex
	data map[reflect.Type]Table
}

func newSyncRegistry() *syncRegistry {
	return &syncRegistry{
		data: make(map[reflect.Type]Table),
	}
}

func (r *syncRegistry) clear() {
	r.Lock()
	r.data = make(map[reflect.Type]Table)
	r.Unlock()
}

func (r *syncRegistry) set(t reflect.Type, table Table) {
	r.Lock()
	r.data[t] = table
	r.Unlock()
}

func (r *syncRegistry) get(t reflect.Type) (Table, bool) {
	r.RLock()
	table, ok := r.data[t]
	r.RUnlock()
	return table, ok
}

// Delete registry cleans the registry.
// This would be mainly used in unit testing.
func DeleteRegistry() {
	registry.clear()
}

// Register adds the passed struct to the registry to be able to use gocql
// MapScan methods with struct types.
//
// It maps the columns using the struct tag 'cql' or the lowercase of the
// field name. You can skip the mapping of one field using the tag `cql:"-"`
func Register(i interface{}) {
	register(i)
}

// Map creates a new map[string]interface{} where each member in the map
// is a reference to a field in the struct. This allows to assign values
// to a struct using gocql MapScan.
//
// Given a gocql session, the following code will populate the struct 't'
// with the values in the datastore.
// 	var t MyStruct
// 	query := session.Query("select * from mytable where id = ?", "my-id")
// 	m := cql.Map(&t)
// 	err := query.MapScan(m)
func Map(i interface{}) map[string]interface{} {
	columns, _ := MapTable(i)
	return columns
}

// MapTable creates a new map[string]interface{} where each member in the map
// is a reference to a field in the struct. This allows to assign values
// to a struct using gocql MapScan. MapTable also returns the Table with the
// information about the type.
//
// Given a gocql session, the following code will populate the struct 't'
// with the values in the datastore.
// 	var t MyStruct
// 	query := session.Query("select * from mytable where id = ?", "my-id")
// 	m, _ := cql.MapTable(&t)
// 	err := query.MapScan(m)
func MapTable(i interface{}) (map[string]interface{}, Table) {
	v := structOf(i)
	t := v.Type()

	// Get the table or register on the fly if necessary
	table, ok := registry.get(t)
	if !ok {
		table = register(i)
	}

	columns := make(map[string]interface{})
	for _, col := range table.Columns {
		var field reflect.Value
		for i, p := range col.Position {
			field = v.Field(p)
			// Don't bother prepping for next position on last
			if len(col.Position) == i+1 {
				break
			}
			next := field
			if field.CanAddr() {
				next = field.Addr()
			}
			if next.Kind() != reflect.Struct {
				break
			}
			if next.CanInterface() {
				v = structOf(next.Interface())
			}
		}
		if field.CanAddr() {
			columns[col.Name] = field.Addr().Interface()
		} else {
			columns[col.Name] = field.Interface()
		}
	}
	return columns, table
}

// Bind returns the values of i to bind in insert queries.
func Bind(i interface{}) []interface{} {
	columns, _, _ := BindTable(i)
	return columns
}

// BindTables returns the values of i to bind in insert queries and the Table
// with the information about the type.
func BindTable(i interface{}) ([]interface{}, map[string]interface{}, Table) {
	v := structOf(i)
	t := v.Type()

	// Get the table or register on the fly if necessary
	table, ok := registry.get(t)
	if !ok {
		table = register(i)
	}

	columns := make([]interface{}, len(table.Columns))
	mapping := make(map[string]interface{})
	for i, col := range table.Columns {
		var field reflect.Value
		for i, p := range col.Position {
			field = v.Field(p)
			// Don't bother prepping for next position on last
			if len(col.Position) == i+1 {
				break
			}
			next := field
			if field.CanAddr() {
				next = field.Addr()
			}
			if next.Kind() != reflect.Struct {
				break
			}
			if next.CanInterface() {
				v = structOf(next.Interface())
			}
		}

		columns[i] = field.Interface()
		mapping[col.Name] = columns[i]
	}
	return columns, mapping, table
}

// GetTable returns the Table with the information about the type of i.
func GetTable(i interface{}) Table {
	v := structOf(i)
	t := v.Type()

	// Get the table or register on the fly if necessary
	table, ok := registry.get(t)
	if !ok {
		table = register(i)
	}

	return table
}

func structOf(i interface{}) reflect.Value {
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Struct:
		return v
	case reflect.Ptr, reflect.Interface:
		elem := v.Elem()
		if elem.Kind() == reflect.Struct {
			return elem
		}
	}

	panic("register type is not struct")
}

func register(i interface{}) Table {
	v := structOf(i)
	t := v.Type()

	// Table name defaults to the type name.
	var table Table
	table.Name = t.Name()

	for i, n := 0, t.NumField(); i < n; i++ {
		field := t.Field(i)

		// Embed fields from anonymous structs--but not at the expense of explicit tags
		if field.Anonymous && v.Field(i).CanInterface() {
			_, tt := MapTable(v.Field(i).Interface())
			if len(tt.Name) > 0 && len(table.Name) == 0 {
				table.Name = tt.Name
			}
			if len(tt.KeyColumns) > 0 && len(table.KeyColumns) == 0 {
				table.KeyColumns = tt.KeyColumns
			}
			if len(tt.Columns) > 0 {
				for _, col := range tt.Columns {
					col.Position = append([]int{i}, col.Position...)
					table.Columns = append(table.Columns, col)
				}
			}
		}

		// Get table if available
		name := field.Tag.Get(TAG_TABLE)
		if name != "" {
			table.Name = name
		}

		// Get the key columns
		name = field.Tag.Get(TAG_KEY)
		if name != "" {
			table.KeyColumns = strings.Split(name, ",")
		}

		// Get columns or field name
		name = field.Tag.Get(TAG_COLUMN)
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		if name != "-" {
			table.Columns = append(table.Columns, Column{name, []int{i}})
		}
	}

	// If no key is explicitly given, assume the first field is implicitly the key
	if len(table.KeyColumns) == 0 && len(table.Columns) > 0 {
		table.KeyColumns = []string{table.Columns[0].Name}
	}

	registry.set(t, table)
	return table
}
