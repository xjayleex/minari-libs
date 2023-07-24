// Protocol Buffers - Google's data interchange format
// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package structpb contains generated types for google/protobuf/struct.proto.
//
// The messages (i.e., Value, Struct, and ListValue) defined in struct.proto are
// used to represent arbitrary JSON. The Value message represents a JSON value,
// the Struct message represents a JSON object, and the ListValue message
// represents a JSON array. See https://json.org for more information.
//
// The Value, Struct, and ListValue types have generated MarshalJSON and
// UnmarshalJSON methods such that they serialize JSON equivalent to what the
// messages themselves represent. Use of these types with the
// "google.golang.org/protobuf/encoding/protojson" package
// ensures that they will be serialized as their JSON equivalent.
//
// # Conversion to and from a Go interface
//
// The standard Go "encoding/json" package has functionality to serialize
// arbitrary types to a large degree. The Value.AsInterface, Struct.AsMap, and
// ListValue.AsSlice methods can convert the protobuf message representation into
// a form represented by interface{}, map[string]interface{}, and []interface{}.
// This form can be used with other packages that operate on such data structures
// and also directly with the standard json package.
//
// In order to convert the interface{}, map[string]interface{}, and []interface{}
// forms back as Value, Struct, and ListValue messages, use the NewStruct,
// NewList, and NewValue constructor functions.
//
// # Example usage
//
// Consider the following example JSON object:
//
//	{
//		"firstName": "John",
//		"lastName": "Smith",
//		"isAlive": true,
//		"age": 27,
//		"address": {
//			"streetAddress": "21 2nd Street",
//			"city": "New York",
//			"state": "NY",
//			"postalCode": "10021-3100"
//		},
//		"phoneNumbers": [
//			{
//				"type": "home",
//				"number": "212 555-1234"
//			},
//			{
//				"type": "office",
//				"number": "646 555-4567"
//			}
//		],
//		"children": [],
//		"spouse": null
//	}
//
// To construct a Value message representing the above JSON object:
//
//	m, err := structpb.NewValue(map[string]interface{}{
//		"firstName": "John",
//		"lastName":  "Smith",
//		"isAlive":   true,
//		"age":       27,
//		"address": map[string]interface{}{
//			"streetAddress": "21 2nd Street",
//			"city":          "New York",
//			"state":         "NY",
//			"postalCode":    "10021-3100",
//		},
//		"phoneNumbers": []interface{}{
//			map[string]interface{}{
//				"type":   "home",
//				"number": "212 555-1234",
//			},
//			map[string]interface{}{
//				"type":   "office",
//				"number": "646 555-4567",
//			},
//		},
//		"children": []interface{}{},
//		"spouse":   nil,
//	})
//	if err != nil {
//		... // handle error
//	}
//	... // make use of m as a *structpb.Value
package helpers

import (
	"encoding/base64"
	"reflect"
	"time"
	utf8 "unicode/utf8"

	"github.com/xjayleex/minari-libs/api/proto/messages"
	"github.com/xjayleex/minari-libs/thirdparty/mapstr"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewStruct constructs a Struct from a general-purpose Go map.
// The map keys must be valid UTF-8.
// The map values are converted using NewValue.
func NewStruct(v map[string]interface{}) (*messages.Struct, error) {
	x := &messages.Struct{Data: make(map[string]*messages.Value, len(v))}
	for k, v := range v {
		if !utf8.ValidString(k) {
			return nil, protoimpl.X.NewError("invalid UTF-8 in string: %q", k)
		}
		var err error
		x.Data[k], err = NewValue(v)
		if err != nil {
			return nil, err
		}
	}
	return x, nil
}

// AsMap converts x to a general-purpose Go map.
// The map values are converted by calling Value.AsInterface.
func AsMap(x *messages.Struct) map[string]interface{} {
	vs := make(map[string]interface{})
	for k, v := range x.GetData() {
		vs[k] = AsInterface(v)
	}
	return vs
}

// AsInterface converts x to a general-purpose Go interface.
//
// Calling Value.MarshalJSON and "encoding/json".Marshal on this output produce
// semantically equivalent JSON (assuming no errors occur).
//
// Floating-point values (i.e., "NaN", "Infinity", and "-Infinity") are
// converted as strings to remain compatible with MarshalJSON.
func AsInterface(x *messages.Value) interface{} {
	switch v := x.GetKind().(type) {
	case *messages.Value_Float64Value:
		if v != nil {
			return v.Float64Value
		}
	case *messages.Value_Float32Value:
		if v != nil {
			return v.Float32Value
		}
	case *messages.Value_Int32Value:
		if v != nil {
			return v.Int32Value
		}
	case *messages.Value_Int64Value:
		if v != nil {
			return v.Int64Value
		}
	case *messages.Value_Uint32Value:
		if v != nil {
			return v.Uint32Value
		}
	case *messages.Value_Uint64Value:
		if v != nil {
			return v.Uint64Value
		}
	case *messages.Value_StringValue:
		if v != nil {
			return v.StringValue
		}
	case *messages.Value_TimestampValue:
		if v != nil {
			return v.TimestampValue.AsTime()
		}
	case *messages.Value_BoolValue:
		if v != nil {
			return v.BoolValue
		}
	case *messages.Value_StructValue:
		if v != nil {
			return AsMap(v.StructValue)
		}
	case *messages.Value_ListValue:
		if v != nil {
			return AsSlice(v.ListValue)
		}
	}
	return nil
}

// AsSlice converts x to a general-purpose Go slice.
// The slice elements are converted by calling Value.AsInterface.
func AsSlice(x *messages.ListValue) []interface{} {
	vs := make([]interface{}, len(x.GetValues()))
	for i, v := range x.GetValues() {
		vs[i] = AsInterface(v)
	}
	return vs
}

// NewValue constructs a Value from a general-purpose Go interface.
// When converting an int64 or uint64 to a NumberValue, numeric precision loss
// is possible since they are stored as a float64.
func NewValue(newValue interface{}) (*messages.Value, error) {

	if newValue == nil {
		return NewNullValue(), nil
	}

	switch newValueTyped := newValue.(type) {
	case bool:
		return NewBoolValue(newValueTyped), nil
	case int:
		return NewInt64Value(int64(newValueTyped)), nil
	case int32:
		return NewInt32Value(newValueTyped), nil
	case int64:
		return NewInt64Value(newValueTyped), nil
	case uint:
		return NewUint64Value(uint64(newValueTyped)), nil
	case uint32:
		return NewUint32Value(newValueTyped), nil
	case uint64:
		return NewUint64Value(newValueTyped), nil
	case float32:
		return NewFloat32Value(newValueTyped), nil
	case float64:
		return NewFloat64Value(newValueTyped), nil
	case string:
		if !utf8.ValidString(newValueTyped) {
			return nil, protoimpl.X.NewError("invalid UTF-8 in string: %q", newValueTyped)
		}
		return NewStringValue(newValueTyped), nil
	case time.Time:
		return NewTimestampValue(newValueTyped), nil

	case map[string]interface{}:
		sv, err := NewStruct(newValueTyped)
		if err != nil {
			return nil, protoimpl.X.NewError("error creating struct object: %q", newValueTyped)
		}
		return NewStructValue(sv), nil
	case mapstr.M: // mapstr.M is just a map[string]interface, but the typecast won't recognize that
		sv, err := NewStruct(newValueTyped)
		if err != nil {
			return nil, protoimpl.X.NewError("error creating struct object: %q", newValueTyped)
		}
		return NewStructValue(sv), nil
	case []interface{}:
		lst, err := NewList(newValueTyped)
		if err != nil {
			return nil, protoimpl.X.NewError("error creating list object: %q", newValueTyped)
		}
		return NewListValue(lst), nil
	case []string: // not strictly needed, but []string seems to be common in log events, so this will give a slight performance boost
		strListVal := &messages.ListValue{Values: make([]*messages.Value, len(newValueTyped))}
		for i, sv := range newValueTyped {
			strListVal.Values[i] = NewStringValue(sv)
		}
		return NewListValue(strListVal), nil
	case []byte:
		s := base64.StdEncoding.EncodeToString(newValueTyped)
		return NewStringValue(s), nil

	default: // fall back to using reflection to unpack the value
		switch reflect.TypeOf(newValueTyped).Kind() {
		case reflect.Struct:
			mapVal := reflect.ValueOf(newValueTyped)
			fields := reflect.TypeOf(newValueTyped)
			interMap := map[string]*messages.Value{}
			for i := 0; i < mapVal.NumField(); i++ {
				msgVal, err := NewValue(mapVal.Field(i).Interface())
				if err != nil {
					return nil, protoimpl.X.NewError("could not convert value of type %T in struct: %s", newValueTyped, err)
				}
				name := fields.Field(i).Name // is there a struct tag we should use instead?
				interMap[name] = msgVal
			}
			structObj := &messages.Struct{Data: interMap}
			return NewStructValue(structObj), nil
		case reflect.Map: // we'll only end up here if we have a map that doesn't resolve to value type interface{}
			reflected := map[string]*messages.Value{}
			mapIter := reflect.ValueOf(newValueTyped).MapRange()
			// hard error if the key type isn't a string
			if reftype := reflect.TypeOf(newValueTyped).Key().Kind(); reftype != reflect.String {
				return nil, protoimpl.X.NewError("maps must have key of type string, got %v", reftype)
			}
			var err error
			for mapIter.Next() {
				k := mapIter.Key().String()
				mv := mapIter.Value().Interface()
				reflected[k], err = NewValue(mv)
				if err != nil {
					protoimpl.X.NewError("could not convert value of type %T in map: %s", mv, err)
				}
			}
			mapObj := &messages.Struct{Data: reflected}
			return NewStructValue(mapObj), nil
		case reflect.Slice: // only for arrays that aren't type []string or []interface{}
			refVal := reflect.ValueOf(newValueTyped)
			listVal := &messages.ListValue{Values: make([]*messages.Value, refVal.Len())}
			for i := 0; i < refVal.Len(); i++ {
				var err error
				listVal.Values[i], err = NewValue(refVal.Index(i).Interface())
				if err != nil {
					return nil, protoimpl.X.NewError("error unpacking field of type %T in array %#v: %s", refVal.Field(i).Interface(), newValueTyped, err)
				}
			}

			return NewListValue(listVal), nil
		default:
			return nil, protoimpl.X.NewError("invalid type: %T", newValueTyped)
		}

	}
}

// NewNullValue constructs a new null Value.
func NewNullValue() *messages.Value {
	return &messages.Value{Kind: &messages.Value_NullValue{NullValue: messages.NullValue_NULL_VALUE}}
}

// NewBoolValue constructs a new boolean Value.
func NewBoolValue(v bool) *messages.Value {
	return &messages.Value{Kind: &messages.Value_BoolValue{BoolValue: v}}
}

// NewFloat32Value constructs a new float32 value
func NewFloat32Value(v float32) *messages.Value {
	return &messages.Value{Kind: &messages.Value_Float32Value{Float32Value: v}}
}

// NewFloat64Value constructs a new float64 value
func NewFloat64Value(v float64) *messages.Value {
	return &messages.Value{Kind: &messages.Value_Float64Value{Float64Value: v}}
}

// NewInt32Value constructs a new int32 value
func NewInt32Value(v int32) *messages.Value {
	return &messages.Value{Kind: &messages.Value_Int32Value{Int32Value: v}}
}

// NewInt64Value constructs a new int64 value
func NewInt64Value(v int64) *messages.Value {
	return &messages.Value{Kind: &messages.Value_Int64Value{Int64Value: v}}
}

// NewUint32Value constructs a new uint632 value
func NewUint32Value(v uint32) *messages.Value {
	return &messages.Value{Kind: &messages.Value_Uint32Value{Uint32Value: v}}
}

// NewUint64Value constructs a new uint64 value
func NewUint64Value(v uint64) *messages.Value {
	return &messages.Value{Kind: &messages.Value_Uint64Value{Uint64Value: v}}
}

// NewStringValue constructs a new string Value.
func NewStringValue(v string) *messages.Value {
	return &messages.Value{Kind: &messages.Value_StringValue{StringValue: v}}
}

// NewTimestampValue constructs a new Timestamp Value.
func NewTimestampValue(v time.Time) *messages.Value {
	return &messages.Value{Kind: &messages.Value_TimestampValue{TimestampValue: timestamppb.New(v)}}
}

// NewStructValue constructs a new struct Value.
func NewStructValue(v *messages.Struct) *messages.Value {
	return &messages.Value{Kind: &messages.Value_StructValue{StructValue: v}}
}

// NewListValue constructs a new list Value.
func NewListValue(v *messages.ListValue) *messages.Value {
	return &messages.Value{Kind: &messages.Value_ListValue{ListValue: v}}
}

// NewList constructs a ListValue from a general-purpose Go slice.
// The slice elements are converted using NewValue.
func NewList(v []interface{}) (*messages.ListValue, error) {
	x := &messages.ListValue{Values: make([]*messages.Value, len(v))}
	for i, v := range v {
		var err error
		x.Values[i], err = NewValue(v)
		if err != nil {
			return nil, err
		}
	}
	return x, nil
}
