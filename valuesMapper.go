package godmin

import (
	"fmt"
	"reflect"
)

func ValuesMapper(values []string, item interface{}) (out map[string]string) {
	itemValue := reflect.ValueOf(item)
	out = make(map[string]string)
	for _, field := range values {
		fieldInterface := reflect.Indirect(itemValue.FieldByName(field)).Interface()
		v, ok := fieldInterface.(fmt.Stringer)
		if ok {
			out[field] = v.String()
		} else {
			// out[field] = fieldInterface
			out[field] = fmt.Sprintf("%v", fieldInterface)
		}
	}
	return out
}
