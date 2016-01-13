package godmin

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type AdminField struct {
	Identifier string
	Type       string
	Value      string
	Children   []AdminField
	ReadOnly   bool
	Omit       bool
	List       bool
}

func existsIn(name string, reference map[string]bool) bool {
	_, ok := reference[name]
	return ok
}

// interface{} in must be a struct
func Marshal(in interface{}, admin ModelAdmin, idPrefix string) []AdminField {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	var out []AdminField
	if idPrefix != "" {
		idPrefix += "."
	}
	for i := 0; i < v.NumField(); i++ {
		af := AdminField{}
		name := v.Type().Field(i).Name
		af.Identifier = idPrefix + name
		if existsIn(name, admin.ListFields) {
			af.List = true
		}
		if existsIn(name, admin.OmitFields) {
			af.Omit = true
		}
		if existsIn(name, admin.ReadOnlyFields) {
			af.ReadOnly = true
		}

		var value string
		kind := v.Field(i).Kind()
		field := v.Field(i)
		// dereference pointers
		if kind == reflect.Ptr {
			kind = v.Field(i).Elem().Kind()
			field = v.Field(i).Elem()
		}
		af.Type = strings.ToLower(kind.String())
		switch kind {
		case reflect.String:
			// check to see if the object has its own String() method (e.g. bson.ObjectId)
			fieldInterface := field.Interface()
			v, ok := fieldInterface.(fmt.Stringer)
			if ok {
				value = v.String()
			} else {
				value = field.String()
			}
		case reflect.Int:
			value = strconv.FormatInt(field.Int(), 10)
		case reflect.Float64:
			value = strconv.FormatFloat(field.Float(), 'f', -1, 64)
		case reflect.Bool:
			value = strconv.FormatBool(field.Bool())
		case reflect.Struct:
			// use Stringer interface if present
			fieldInterface := field.Interface()
			v, ok := fieldInterface.(fmt.Stringer)
			if ok {
				value = v.String()
			} else { // otherwise, nest it
				af.Children = Marshal(field.Interface(), admin, af.Identifier)
			}

		case reflect.Slice:
			for c := 0; c < field.Len(); c++ {
				sliceKind := field.Index(c).Kind()
				if sliceKind == reflect.Ptr {
					sliceKind = field.Index(c).Elem().Kind()
				}
				if sliceKind == reflect.Struct {
					cf := AdminField{}
					cf.Identifier = af.Identifier + "." + strconv.Itoa(c)
					cf.Children = Marshal(field.Index(c).Interface(), admin, cf.Identifier)
					af.Children = append(af.Children, cf)
				}
			}
		default:
			// use Stringer interface if present
			if field.IsValid() {
				fieldInterface := field.Interface()
				v, ok := fieldInterface.(fmt.Stringer)
				if ok {
					value = v.String()
				}
			}
		}
		af.Value = value
		out = append(out, af)
	}
	return out
}

// Unmarshal values with identfiers provided by Marshal into a map[string][]string
// excluding Omit and ReadOnly fields
func Unmarshal(values url.Values, modelAdmin *ModelAdmin) (out map[string][]string) {
	out = make(map[string][]string)
	for key, val := range values {
		if _, skip := modelAdmin.ReadOnlyFields[key]; skip {
			continue
		}
		if _, skip := modelAdmin.OmitFields[key]; skip {
			continue
		}
		out[key] = val
	}
	return
}
