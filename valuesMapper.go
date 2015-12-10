package godmin

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Maps arbitrary values to string representations for display in HTML
func ValuesMapper(item interface{}) (out map[string]string) {
	itemValue := reflect.ValueOf(item)
	out = make(map[string]string)
	if itemKind := itemValue.Kind(); itemKind != reflect.Struct {
		return
	}
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
				switch val.Kind() {
				case reflect.Slice, reflect.Struct:
					jv, err := json.MarshalIndent(val.Interface(), "", "  ")
					if err != nil {
						fmt.Println(err)
					} else {
						out[fieldName] = fmt.Sprintf("%s", jv)
					}
				default:
					out[fieldName] = fmt.Sprintf("%v", fieldInterface)
				}
			}
		} else {
			out[fieldName] = ""
		}
	}
	return out
}
