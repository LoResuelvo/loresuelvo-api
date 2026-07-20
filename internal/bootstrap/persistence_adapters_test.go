package bootstrap

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestNewPersistenceAdaptersInitializesEveryAdapter(t *testing.T) {
	adapters := NewPersistenceAdapters(&sql.DB{})
	value := reflect.ValueOf(adapters).Elem()
	adaptersType := value.Type()

	for index := range value.NumField() {
		if value.Field(index).IsNil() {
			t.Errorf("expected %s to be initialized", adaptersType.Field(index).Name)
		}
	}
}
