package godmin

import (
	"reflect"
)

func defaultWidgets(ma ModelAdmin) (widgets map[string]string) {
	proto := ma.Accessor.Prototype()
	itemValue := reflect.ValueOf(proto)
	widgets = make(map[string]string)
	itemType := itemValue.Type()
	for i := 0; i < itemValue.NumField(); i++ {
		fieldName := itemType.Field(i).Name
		fieldKind := itemValue.Field(i).Interface()
		widgets[fieldName] = "text"
		switch fieldKind.(type) {
		case *bool:
			widgets[fieldName] = "radio"
		case bool:
			widgets[fieldName] = "radio"
		}
		switch itemType.Field(i).Type.Kind() {
		case reflect.Struct, reflect.Slice:
			widgets[fieldName] = "textarea"
		}
	}
	return
}
