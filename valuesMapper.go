package godmin

import (
	"fmt"
	"reflect"
)

func ValuesMapper(values []string, item interface{}) (out map[string]string) {
	itemValue := reflect.ValueOf(item)
	out = make(map[string]string)
	var fieldInterface interface{}
	var val reflect.Value
	for _, field := range values {
		val = reflect.Indirect(itemValue.FieldByName(field))
		if val.IsValid() {
			fieldInterface = val.Interface()
			v, ok := fieldInterface.(fmt.Stringer)
			if ok {
				out[field] = v.String()
			} else {
				// out[field] = fieldInterface
				out[field] = fmt.Sprintf("%v", fieldInterface)
			}
		} else {
			out[field] = ""
		}
	}
	return out
}
