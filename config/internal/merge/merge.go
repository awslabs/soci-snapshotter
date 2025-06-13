/*
   Copyright The Soci Snapshotter Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package merge

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	structTOMLAnnotation = "toml"

	// Arbitrarily selected based on existing use-case depth. Limiting recursion depth protects against
	// stack overflows when handling malicious or malformed inputs. Assuming a 400-600 byte implementation
	// of the merge function and a 1MB stack size, then in the worst case the depth limit would allow
	// 2-3% consumption of the stack (excluding factors like garbage collection and runtime).
	maxRecursionDepth = 50
)

var (
	ErrDstNotStruct     = errors.New("dst must be a struct or a pointer to a struct")
	ErrCannotCast       = errors.New("cannot cast value")
	ErrExceededMaxDepth = fmt.Errorf("exceeded maximum recursion depth of %d", maxRecursionDepth)
)

func Merge(dst any, src *map[string]any) error {
	return mergeWithDepth(dst, src, 0)
}

func mergeWithDepth(dst any, src *map[string]any, depth int) error {
	if dst == nil || src == nil {
		return errors.New("src and dst must not be nil")
	}

	if depth > maxRecursionDepth {
		return ErrExceededMaxDepth
	}

	dstVal := reflect.ValueOf(dst)

	// Destination should be a pointer to a struct.
	if dstVal.Kind() == reflect.Ptr {
		dstVal = dstVal.Elem()
	}
	if dstVal.Kind() != reflect.Struct {
		// Gracefully error here.
		// [reflect.Type].NumField() will panic if type is not struct.
		return ErrDstNotStruct
	}

	dstType := dstVal.Type()
	for i := range dstType.NumField() {
		fieldVal := dstVal.Field(i)
		fieldType := dstType.Field(i)

		tomlAnnotation := fieldType.Tag.Get(structTOMLAnnotation)
		fieldKind := fieldType.Type.Kind()

		if tomlAnnotation == "" {
			// A struct field with no tag could be an embedded struct that contains more fields at the same map level.
			if fieldKind == reflect.Struct {
				if err := mergeWithDepth(fieldVal.Addr().Interface(), src, depth+1); err != nil {
					return err
				}
			}
			continue
		}

		// Handle multiple TOML annotations separated by commas
		annotations := strings.Split(tomlAnnotation, ",")
		var srcAnyVal any
		var ok bool

		// Try each annotation until we find a match
		for _, annotation := range annotations {
			annotation = strings.TrimSpace(annotation)
			if srcAnyVal, ok = (*src)[annotation]; ok {
				tomlAnnotation = annotation
				break
			}
		}

		if ok {
			switch fieldKind {
			case reflect.Struct:
				srcMapStringAny, ok := srcAnyVal.(map[string]any)
				if !ok {
					return fmt.Errorf("value of %q from src cannot be cast to %T", tomlAnnotation, dst)
				}

				if err := mergeWithDepth(fieldVal.Addr().Interface(), &srcMapStringAny, depth+1); err != nil {
					return err
				}
			case reflect.Map:
				if fieldVal.IsNil() {
					fieldVal.Set(reflect.MakeMap(fieldVal.Type()))
				}

				srcMapStringAny, ok := srcAnyVal.(map[string]any)
				if ok {
					if err := handleMap(fieldVal, srcMapStringAny, depth); err != nil {
						return err
					}
					continue
				}

				if err := setValue(fieldVal, srcAnyVal); err != nil {
					return fmt.Errorf("error setting value of %q: %w", tomlAnnotation, err)
				}
			case reflect.Slice:
				srcSliceAny, ok := srcAnyVal.([]any)
				if !ok {
					return fmt.Errorf("value of %q from src cannot be cast to []any", tomlAnnotation)
				}

				if err := handleSlice(fieldVal, srcSliceAny, depth); err != nil {
					return err
				}
			default:
				if err := setValue(fieldVal, srcAnyVal); err != nil {
					return fmt.Errorf("error setting value of %q: %w", tomlAnnotation, err)
				}
			}
		}
	}

	return nil
}

// handleMap processes a map of interfaces and sets it to the destination field
func handleMap(dst reflect.Value, srcMap map[string]any, depth int) error {
	keyType := dst.Type().Key()
	elemType := dst.Type().Elem()
	for k, v := range srcMap {
		keyVal := reflect.ValueOf(k)
		if !keyVal.Type().AssignableTo(keyType) && !keyVal.Type().ConvertibleTo(keyType) {
			return fmt.Errorf("map key cannot be cast to %s", keyType.String())
		}

		if !keyVal.Type().AssignableTo(keyType) {
			keyVal = keyVal.Convert(keyType)
		}

		switch val := v.(type) {
		case []any:
			newSliceVal := reflect.New(elemType).Elem()

			if err := handleSlice(newSliceVal, val, depth); err != nil {
				return err
			}

			dst.SetMapIndex(keyVal, newSliceVal)
		case map[string]any:
			newElem := reflect.New(elemType).Elem()

			// Recursively merge the map into the struct
			if err := mergeWithDepth(newElem.Addr().Interface(), &val, depth+1); err != nil {
				return err
			}

			dst.SetMapIndex(keyVal, newElem)
		default:
			return errors.New("map value cannot be cast")
		}
	}
	return nil
}

// handleSlice processes a slice of interfaces and sets it to the destination field
func handleSlice(dst reflect.Value, srcSlice []interface{}, depth int) error {
	elemType := dst.Type().Elem()
	newSlice := reflect.MakeSlice(dst.Type(), len(srcSlice), len(srcSlice))

	for i, v := range srcSlice {
		// Special handling for struct elements in slices
		if elemType.Kind() == reflect.Struct {
			if srcMap, ok := v.(map[string]any); ok {
				// Create a new instance of the struct type
				newElem := reflect.New(elemType).Elem()
				// Recursively merge the map into the struct
				if err := mergeWithDepth(newElem.Addr().Interface(), &srcMap, depth+1); err != nil {
					return err
				}
				newSlice.Index(i).Set(newElem)
				continue
			}
		}

		// Regular handling for non-struct elements
		if err := setValue(newSlice.Index(i), v); err != nil {
			return err
		}
	}
	dst.Set(newSlice)
	return nil
}

// setValue sets a reflect.Value to the given value, handling type conversions
func setValue(dst reflect.Value, src any) error {
	srcVal := reflect.ValueOf(src)
	dstType := dst.Type()

	if srcVal.Type().AssignableTo(dstType) {
		dst.Set(srcVal)
		return nil
	}

	if srcVal.Type().ConvertibleTo(dstType) {
		dst.Set(srcVal.Convert(dstType))
		return nil
	}

	return fmt.Errorf("value cannot be cast to %s", dstType.String())
}
