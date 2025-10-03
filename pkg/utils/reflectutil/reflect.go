package reflectutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/common"
)

const tagKey = "scan"

// returns pkg dir (relative to project dir) and virable name
func Locate(obj any) (string, string) {
	ot := reflect.TypeOf(obj)
	if ot == nil {
		return "", ""
	}
	for ot.Kind() == reflect.Pointer {
		ot = ot.Elem()
	}
	return ot.PkgPath(), ot.Name()
}

func ToBytes(value reflect.Value) []byte {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Struct {
		b, _ := json.Marshal(value.Interface())
		return b
	}
	if value.Kind() == reflect.Slice {
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return value.Bytes()
		}
		b, _ := json.Marshal(value.Interface())
		return b
	}
	return []byte(fmt.Sprint(value))
}

func FromBytes(t reflect.Type, b []byte) (reflect.Value, error) {
	isPtr := false
	for t.Kind() == reflect.Pointer {
		isPtr = true
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		v := string(b)
		if isPtr {
			return reflect.ValueOf((*string)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Int:
		i, err := strconv.ParseInt(string(b), 0, 64)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := int(i)
		if isPtr {
			return reflect.ValueOf((*int)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Int8:
		i, err := strconv.ParseInt(string(b), 0, 8)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := int8(i)
		if isPtr {
			return reflect.ValueOf((*int8)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Int16:
		i, err := strconv.ParseInt(string(b), 0, 16)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := int16(i)
		if isPtr {
			return reflect.ValueOf((*int16)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Int32:
		i, err := strconv.ParseInt(string(b), 0, 32)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := int32(i)
		if isPtr {
			return reflect.ValueOf((*int32)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Int64:
		i, err := strconv.ParseInt(string(b), 0, 64)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := i
		if isPtr {
			return reflect.ValueOf((*int64)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Uint:
		u, err := strconv.ParseUint(string(b), 0, 64)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := uint(u)
		if isPtr {
			return reflect.ValueOf((*uint)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Uint8:
		u, err := strconv.ParseUint(string(b), 0, 8)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := uint8(u)
		if isPtr {
			return reflect.ValueOf((*uint8)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Uint16:
		u, err := strconv.ParseUint(string(b), 0, 16)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := uint16(u)
		if isPtr {
			return reflect.ValueOf((*uint16)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Uint32:
		u, err := strconv.ParseUint(string(b), 0, 32)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := uint32(u)
		if isPtr {
			return reflect.ValueOf((*uint32)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Uint64:
		u, err := strconv.ParseUint(string(b), 0, 64)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := u
		if isPtr {
			return reflect.ValueOf((*uint64)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Float32:
		f, err := strconv.ParseFloat(string(b), 32)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := float32(f)
		if isPtr {
			return reflect.ValueOf((*float32)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Float64:
		f, err := strconv.ParseFloat(string(b), 64)
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := f
		if isPtr {
			return reflect.ValueOf((*float64)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(string(b))
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		v := b
		if isPtr {
			return reflect.ValueOf((*bool)(&v)), nil
		} else {
			return reflect.ValueOf(v), nil
		}
	case reflect.Slice:
		// store []byte directly
		if t.Elem().Kind() == reflect.Uint8 {
			return reflect.ValueOf(b), nil
		}
		// store json bytes for other kind of slices
		v := reflect.New(t)
		err := json.Unmarshal(b, v.Interface())
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		if isPtr {
			return v, nil
		} else {
			return v.Elem(), nil
		}
	case reflect.Struct:
		v := reflect.New(t)
		err := json.Unmarshal(b, v.Interface())
		if err != nil {
			return reflect.Zero(t), errors.Wrap(err)
		}
		if isPtr {
			return v, nil
		} else {
			return v.Elem(), nil
		}
	default:
		return reflect.Zero(t), errors.Newf("unsupported field kind %s, must be one of string, int, uint, float, bool w/o pointer or []byte", t.Kind())
	}
}

func Scan(obj any) ([]common.Pair[string, []byte], error) {
	objType := reflect.TypeOf(obj)
	objValue := reflect.ValueOf(obj)
	// fmt.Println("obj type is", objType, "value is", objValue)
	if objType.Kind() == reflect.Pointer {
		if objValue.IsNil() {
			// fmt.Println("obj value is nil")
			return nil, nil
		}
		objType = objType.Elem()
		objValue = objValue.Elem()
		// fmt.Println("obj type is", objType, "value is", objValue)
	}
	if objValue.Kind() != reflect.Struct {
		return nil, errors.Newf("unsupported obj kind: %s", objValue.Kind())
	}
	var result []common.Pair[string, []byte]
	for i := 0; i < objType.NumField(); i++ {
		fieldType := objType.Field(i)
		fieldValue := objValue.Field(i)
		key := fieldType.Name
		// fmt.Println("field name is", key)
		tags := strings.Split(fieldType.Tag.Get(tagKey), ",")
		if len(tags) > 0 {
			// fmt.Println("tags are", tags)
			if tags[0] == "-" {
				continue
			}
		}
		// fmt.Println("type is", fieldType, "value is", fieldValue)
		value := ToBytes(fieldValue)
		// if sliceutil.In(tagEncrypt, tags...) {
		// 	if kp == nil {
		// 		return nil, errors.Newf("failed to encrypt key %s: rsa key does not exists", key)
		// 	}
		// 	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &kp.PublicKey, value)
		// 	if err != nil {
		// 		return nil, errors.Wrap(err)
		// 	}
		// 	value = ciphertext
		// }
		pair := common.NewPair(key, value)
		// fmt.Println("pair is", pair)
		result = append(result, pair)
	}
	return result, nil
}

func Apply(obj any, fields []common.Pair[string, []byte]) error {
	objType := reflect.TypeOf(obj)
	objValue := reflect.ValueOf(obj)
	// fmt.Println("obj type is", objType, "value is", objValue)
	if objType.Kind() == reflect.Pointer {
		if objValue.IsNil() {
			return errors.Newf("obj must be an zero value instead of nil")
		}
		objType = objType.Elem()
		objValue = objValue.Elem()
		// fmt.Println("obj type is", objType, "value is", objValue)
	}
	if objValue.Kind() != reflect.Struct {
		return errors.Newf("unsupported obj kind: %s", objValue.Kind())
	}
	values := make(map[string][]byte)
	for _, field := range fields {
		values[field.GetKey()] = []byte(field.GetValue())
	}
	for i := 0; i < objType.NumField(); i++ {
		fieldType := objType.Field(i)
		key := fieldType.Name
		// fmt.Println("field name is", key)
		// fmt.Println("type is", fieldType.Type)
		tags := strings.Split(fieldType.Tag.Get(tagKey), ",")
		if len(tags) > 0 {
			// fmt.Println("tags are", tags)
			if tags[0] == "-" {
				continue
			}
		}
		value, ok := values[key]
		if !ok || len(value) == 0 {
			continue
		}
		// if sliceutil.In(tagEncrypt, tags...) {
		// 	if kp == nil {
		// 		return errors.Newf("failed to encrypt key %s: rsa key does not exists", key)
		// 	}
		// 	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, kp, value)
		// 	if err != nil {
		// 		return errors.Wrap(err)
		// 	}
		// 	value = plaintext
		// }
		v, err := FromBytes(fieldType.Type, value)
		if err != nil {
			return errors.Wrap(err)
		}
		fieldValue := objValue.Field(i)
		fieldValue.Set(v)
	}
	return nil
}
