package core

import "reflect"

func GetField(value any, fieldName string) (string, bool) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		field := v.FieldByName(fieldName)
		if field.IsValid() && field.CanInterface() {
			if text, ok := field.Interface().(string); ok {
				return text, true
			}
		}
	}
	return "", false
}

func GetTextField(value any) (string, bool) {
	return GetField(value, "text")
}

func HasField(value any, fieldName string) bool {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		field := v.FieldByName(fieldName)
		if field.IsValid() && field.CanInterface() {
			return true
		}
	}
	return false
}
