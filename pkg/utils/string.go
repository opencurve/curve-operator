package utils

import (
	"reflect"
	"strconv"
)

func Type(v interface{}) string {
	switch v.(type) {
	case bool:
		return "bool"
	case string:
		return "string"
	case int:
		return "int"
	case int64:
		return "int64"
	case map[string]interface{}:
		return "string_interface_map"
	default:
		return "unknown"
	}
}

func IsBool(v interface{}) bool {
	return Type(v) == "bool"
}

func IsString(v interface{}) bool {
	return Type(v) == "string"
}

func IsInt(v interface{}) bool {
	return Type(v) == "int"
}

func IsInt64(v interface{}) bool {
	return Type(v) == "int64"
}

func IsStringAnyMap(v interface{}) bool {
	return Type(v) == "string_interface_map"
}

func IsFunc(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Func
}

func All2Str(v interface{}) (value string, ok bool) {
	ok = true
	if IsString(v) {
		value = v.(string)
	} else if IsInt(v) {
		value = strconv.Itoa(v.(int))
	} else if IsBool(v) {
		value = strconv.FormatBool(v.(bool))
	} else {
		ok = false
	}
	return
}

// convert all to string
func Atoa(v interface{}) string {
	value, _ := All2Str(v)
	return value
}

func Str2Int(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	return v, err == nil
}

func Str2Bool(s string) (bool, bool) { // value, ok
	v, err := strconv.ParseBool(s)
	return v, err == nil
}

func IsTrueStr(s string) bool {
	v, yes := Str2Bool(s)
	return yes && v == true
}
