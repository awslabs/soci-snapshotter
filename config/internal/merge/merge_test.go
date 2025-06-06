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
	"reflect"
	"testing"
)

type EmbeddedStruct struct {
	IntField int `toml:"embedded_int_field"`
}

type Example struct {
	IntField    int         `toml:"int_field"`
	FloatField  float64     `toml:"float_field"`
	BoolField   bool        `toml:"bool_field"`
	StringField string      `toml:"string_field"`
	Slice       []string    `toml:"slice"`
	Map         map[any]any `toml:"map"`
	Struct      Struct      `toml:"struct"`
	EmbeddedStruct

	MultipleAnnotationField int `toml:"MultipleAnnotationField,multiple_annocation_field"`
}

type Struct struct {
	IntField    int                      `toml:"int_field"`
	FloatField  float64                  `toml:"float_field"`
	BoolField   bool                     `toml:"bool_field"`
	StringField string                   `toml:"string_field"`
	Slice       []string                 `toml:"slice"`
	Map         map[any]any              `toml:"map"`
	ComplexMap  map[string]ComplexStruct `toml:"complex_map"`
}

type ComplexStruct struct {
	ComplexSlice []SimpleStruct `toml:"complex_slice"`
}

type SimpleStruct struct {
	SimpleField string `toml:"simple_field"`
}

func TestMerge(t *testing.T) {
	for _, test := range []struct {
		name      string
		dst       *Example
		src       *map[string]any
		expected  *Example
		expectErr bool
	}{
		{
			name: "dst(int)",
			dst:  &Example{},
			src: &map[string]any{
				"int_field": 10,
			},
			expected: &Example{
				IntField: 10,
			},
		},
		{
			name: "src(int)",
			dst: &Example{
				IntField: 10,
			},
			src: &map[string]any{},
			expected: &Example{
				IntField: 10,
			},
		},
		{
			name: "overwrite(int)",
			dst: &Example{
				IntField: 10,
			},
			src: &map[string]any{
				"int_field": 20,
			},
			expected: &Example{
				IntField: 20,
			},
		},
		{
			name: "dst(float)",
			dst:  &Example{},
			src: &map[string]any{
				"float_field": 10.0,
			},
			expected: &Example{
				FloatField: 10.0,
			},
		},
		{
			name: "src(float)",
			dst: &Example{
				FloatField: 10.0,
			},
			src: &map[string]any{},
			expected: &Example{
				FloatField: 10.0,
			},
		},
		{
			name: "overwrite(float)",
			dst: &Example{
				FloatField: 10.0,
			},
			src: &map[string]any{
				"float_field": 20.0,
			},
			expected: &Example{
				FloatField: 20.0,
			},
		},
		{
			name: "dst(bool)",
			dst:  &Example{},
			src: &map[string]any{
				"bool_field": true,
			},
			expected: &Example{
				BoolField: true,
			},
		},
		{
			name: "src(bool)",
			dst: &Example{
				BoolField: true,
			},
			src: &map[string]any{},
			expected: &Example{
				BoolField: true,
			},
		},
		{
			name: "overwrite(bool)",
			dst: &Example{
				BoolField: true,
			},
			src: &map[string]any{
				"bool_field": false,
			},
			expected: &Example{
				BoolField: false,
			},
		},
		{
			name: "dst(string)",
			dst:  &Example{},
			src: &map[string]any{
				"string_field": "some string",
			},
			expected: &Example{
				StringField: "some string",
			},
		},
		{
			name: "src(string)",
			dst: &Example{
				StringField: "some string",
			},
			src: &map[string]any{},
			expected: &Example{
				StringField: "some string",
			},
		},
		{
			name: "overwrite(string)",
			dst: &Example{
				StringField: "some string",
			},
			src: &map[string]any{
				"string_field": "other string",
			},
			expected: &Example{
				StringField: "other string",
			},
		},
		{
			name: "dst(slice)",
			dst:  &Example{},
			src: &map[string]any{
				"slice": []any{"some", "strings"},
			},
			expected: &Example{
				Slice: []string{"some", "strings"},
			},
		},
		{
			name: "src(slice)",
			dst: &Example{
				Slice: []string{"some", "strings"},
			},
			src: &map[string]any{},
			expected: &Example{
				Slice: []string{"some", "strings"},
			},
		},
		{
			name: "overwrite(slice)",
			dst: &Example{
				Slice: []string{"some", "strings"},
			},
			src: &map[string]any{
				"slice": []any{"other", "strings"},
			},
			expected: &Example{
				Slice: []string{"other", "strings"},
			},
		},
		{
			name: "src(map)",
			dst:  &Example{},
			src: &map[string]any{
				"map": map[any]any{
					"some": "map",
				},
			},
			expected: &Example{
				Map: map[any]any{
					"some": "map",
				},
			},
		},
		{
			name: "dst(map)",
			dst: &Example{
				Map: map[any]any{
					"some": "map",
				},
			},
			src: &map[string]any{},
			expected: &Example{
				Map: map[any]any{
					"some": "map",
				},
			},
		},
		{
			name: "overwrite(map)",
			dst: &Example{
				Map: map[any]any{
					"some": "map",
				},
			},
			src: &map[string]any{
				"map": map[any]any{
					"other": "map",
				},
			},
			expected: &Example{
				Map: map[any]any{
					"other": "map",
				},
			},
		},
		{
			name: "src(struct)",
			dst:  &Example{},
			src: &map[string]any{
				"struct": map[string]any{
					"int_field": 10,
				},
			},
			expected: &Example{
				Struct: Struct{
					IntField: 10,
				},
			},
		},
		{
			name: "dst(struct)",
			dst: &Example{
				Struct: Struct{
					IntField: 10,
				},
			},
			src: &map[string]any{},
			expected: &Example{
				Struct: Struct{
					IntField: 10,
				},
			},
		},
		{
			name: "overwrite(struct)",
			dst: &Example{
				Struct: Struct{
					IntField: 10,
				},
			},
			src: &map[string]any{
				"struct": map[string]any{
					"int_field": 20,
				},
			},
			expected: &Example{
				Struct: Struct{
					IntField: 20,
				},
			},
		},
		{
			name: "merge(struct)",
			dst: &Example{
				Struct: Struct{
					IntField: 10,
				},
			},
			src: &map[string]any{
				"struct": map[string]any{
					"string_field": "some string",
				},
			},
			expected: &Example{
				Struct: Struct{
					IntField:    10,
					StringField: "some string",
				},
			},
		},
		{
			name: "dst(embedded_config)",
			dst:  &Example{},
			src: &map[string]any{
				"embedded_int_field": 10,
			},
			expected: &Example{
				EmbeddedStruct: EmbeddedStruct{
					IntField: 10,
				},
			},
		},
		{
			name: "src(embedded_config)",
			dst: &Example{
				EmbeddedStruct: EmbeddedStruct{
					IntField: 10,
				},
			},
			src: &map[string]any{},
			expected: &Example{
				EmbeddedStruct: EmbeddedStruct{
					IntField: 10,
				},
			},
		},
		{
			name: "overwrite(embedded_config)",
			dst: &Example{
				EmbeddedStruct: EmbeddedStruct{
					IntField: 10,
				},
			},
			src: &map[string]any{
				"embedded_int_field": 20,
			},
			expected: &Example{
				EmbeddedStruct: EmbeddedStruct{
					IntField: 20,
				},
			},
		},
		{
			name: "overwrite(1st_annotation)",
			dst: &Example{
				MultipleAnnotationField: 10,
			},
			src: &map[string]any{
				"MultipleAnnotationField": 20,
			},
			expected: &Example{
				MultipleAnnotationField: 20,
			},
		},
		{
			name: "overwrite(2nd_annotation)",
			dst: &Example{
				MultipleAnnotationField: 10,
			},
			src: &map[string]any{
				"multiple_annocation_field": 20,
			},
			expected: &Example{
				MultipleAnnotationField: 20,
			},
		},
		{
			name: "complex",
			dst:  &Example{},
			src: &map[string]any{
				"struct": map[string]any{
					"complex_map": map[string]any{
						"complex_struct": map[string]any{
							"complex_slice": []any{
								map[string]any{
									"simple_field": "value",
								},
							},
						},
					},
				},
			},
			expected: &Example{
				Struct: Struct{
					ComplexMap: map[string]ComplexStruct{
						"complex_struct": {
							ComplexSlice: []SimpleStruct{
								{
									SimpleField: "value",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "negative(src=nil)",
			dst:       &Example{},
			src:       nil,
			expected:  &Example{},
			expectErr: true,
		},
		{
			name:      "negative(dst=nil)",
			dst:       nil,
			src:       &map[string]any{},
			expectErr: true,
		},
		{
			name: "negative(cast error)",
			dst:  &Example{},
			src: &map[string]any{
				"int_field": "some string",
			},
			expected:  &Example{},
			expectErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Merge(test.dst, test.src)
			if err != nil && !test.expectErr {
				t.Fatalf("unexpected error: %v", err)
			} else if err == nil && test.expectErr {
				t.Fatal("expected error")
			} else if !reflect.DeepEqual(test.expected, test.dst) {
				t.Fatalf("invalid config.\nexpected: %v\n  actual: %v", test.expected, test.dst)
			}
		})
	}

	t.Run("negative(dst=value)", func(t *testing.T) {
		if err := Merge(10, &map[string]any{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("negative(exceeds depth limit)", func(t *testing.T) {
		if err := mergeWithDepth(&Example{}, &map[string]any{}, maxRecursionDepth+1); err == nil {
			t.Fatal("expected error")
		}
	})
}

func BenchmarkMerge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Merge(&Example{}, &map[string]any{
			"int_field":    10,
			"float_field":  10.0,
			"bool_field":   true,
			"string_field": "some string",
			"slice":        []any{"some", "strings"},
			"map": map[any]any{
				"some": "map",
			},
			"struct": map[string]any{
				"int_field":    10,
				"float_field":  10.0,
				"bool_field":   true,
				"string_field": "some string",
				"slice":        []any{"some", "strings"},
				"map": map[any]any{
					"some": "map",
				},
				"complex_map": map[string]any{
					"complex_struct": map[string]any{
						"complex_slice": []any{
							map[string]any{
								"simple_field": "value",
							},
						},
					},
				},
			},
			"embedded_int_field":      10,
			"MultipleAnnotationField": 10,
		})
	}
}
