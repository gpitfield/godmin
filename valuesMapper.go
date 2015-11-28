package godmin

import (
	"fmt"
	"reflect"
)

// Maps arbitrary values to string representations for display in HTML
func ValuesMapper(item interface{}) (out map[string]string) {
	itemValue := reflect.ValueOf(item)
	out = make(map[string]string)
	var fieldInterface interface{}
	var val reflect.Value
	itemType := itemValue.Type()
	for i := 0; i < itemValue.NumField(); i++ {
		fieldName := itemType.Field(i).Name
		val = reflect.Indirect(itemValue.FieldByName(fieldName))
		if val.IsValid() {
			fieldInterface = val.Interface()
			v, ok := fieldInterface.(fmt.Stringer)
			if ok {
				out[fieldName] = v.String()
			} else {
				out[fieldName] = fmt.Sprintf("%v", fieldInterface)
			}
		} else {
			out[fieldName] = ""
		}
	}
	return out
}
