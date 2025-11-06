package api

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseExpr generates an expr-lang compatible map[string]any, with all fields present and collections instantiated.
// - `yaml` struct tags will be modified to drop `omitempty`.
// - All non-yaml tags will be removed.
// - If the type (or sub-types) implement the YAML or Text marshallers, then they will be copied as-is to preserve the methods.
// - If the type contains recursive types, then this function will return an error as Go does not allow recursive anonymous types.
func ParseExpr(obj any) (map[string]any, error) {
	// Removes `omitempty` from the yaml tag, or overrides it entirely with the `expr` tag if present.
	exprCompatibleTags := func(oldTag reflect.StructTag) reflect.StructTag {
		args := []string{}
		for arg := range strings.SplitSeq(oldTag.Get("yaml"), ",") {
			if arg != "omitempty" {
				args = append(args, arg)
			}
		}

		if len(args) > 0 {
			tag := `yaml:"` + strings.Join(args, ",") + `"`
			if tag != oldTag.Get("yaml") {
				return reflect.StructTag(tag)
			}
		}

		return oldTag
	}

	// Recursively creates a new struct with mangled tags.
	recursiveTypes := map[reflect.Type]reflect.Type{}
	var overrideTags func(t reflect.Type) reflect.Type
	overrideTags = func(t reflect.Type) reflect.Type {
		for i := 0; i < t.NumMethod(); i++ {
			method := t.Method(i)
			// Don't mangle types with custom marshallers. These types' fields' `omitempty` tags will persist.
			if slices.Contains([]string{"UnmarshalYAML", "UnmarshalText", "MarshalYAML", "MarshalText"}, method.Name) {
				return t
			}
		}

		switch t.Kind() {
		case reflect.Struct:
			fields := []reflect.StructField{}
			var unchanged int
			_, ok := recursiveTypes[t]
			if ok {
				recursiveTypes[nil] = t
				return t
			}

			recursiveTypes[t] = t
			defer delete(recursiveTypes, t)
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				if field.IsExported() {
					// If the type is a pointer to a struct, preserve its tags so we don't end up with a `nil` value, which expr can't handle for structs.
					if field.Type.Kind() != reflect.Pointer || field.Type.Elem().Kind() != reflect.Struct {
						field.Tag = exprCompatibleTags(field.Tag)
					}

					if field.Tag.Get("yaml") != "" {
						field.Type = overrideTags(field.Type)
					}
				}

				fields = append(fields, field)
				if t.Field(i).Type == fields[i].Type && t.Field(i).Tag == fields[i].Tag {
					unchanged++
				}
			}

			// Try to preserve the underlying type if we didn't mangle anything.
			if unchanged == t.NumField() {
				return t
			}

			return reflect.StructOf(fields)
		case reflect.Slice:
			return reflect.SliceOf(overrideTags(t.Elem()))
		case reflect.Map:
			return reflect.MapOf(t.Key(), overrideTags(t.Elem()))
		case reflect.Pointer:
			return reflect.PointerTo(overrideTags(t.Elem()))
		default:
			return t
		}
	}

	if reflect.TypeOf(obj) == nil {
		return nil, fmt.Errorf("Object type is nil")
	}

	b, err := yaml.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal object as yaml: %w", err)
	}

	newObj := reflect.New(overrideTags(reflect.TypeOf(obj))).Interface()
	err = yaml.Unmarshal(b, newObj)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal object after modifying tags: %w", err)
	}

	rec, ok := recursiveTypes[nil]
	if ok {
		return nil, fmt.Errorf("Type \"%T\" has recursive type: %q", obj, rec)
	}

	b, err = yaml.Marshal(newObj)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal modified object as yaml: %w", err)
	}

	out := map[string]any{}
	err = yaml.Unmarshal(b, &out)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal object as map: %w", err)
	}

	return out, nil
}
