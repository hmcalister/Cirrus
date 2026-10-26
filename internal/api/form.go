package api

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// The media type that htmx forms post by default. Huma ships JSON and multipart
// codecs only, so this project registers a form codec to keep the server-rendered
// forms working while still letting Huma parse a typed body struct.
const formMediaType = "application/x-www-form-urlencoded"

func newFormFormat() huma.Format {
	return huma.Format{
		Marshal: func(io.Writer, any) error {
			return errors.New("application/x-www-form-urlencoded responses are not supported")
		},
		Unmarshal: unmarshalForm,
	}
}

// Unmarshals a form body into v.
//
// Huma calls this twice: once with a *any to validate the body against the
// OpenAPI schema, and once with a pointer to the input Body struct.
func unmarshalForm(data []byte, v any) error {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("form target must be a non-nil pointer")
	}
	rv = rv.Elem()

	if rv.Kind() == reflect.Interface {
		parsed := make(map[string]any, len(values))
		for key, vals := range values {
			if len(vals) == 1 {
				parsed[key] = vals[0]
			} else {
				parsed[key] = vals
			}
		}
		rv.Set(reflect.ValueOf(parsed))
		return nil
	}

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("form target must be a struct, got %s", rv.Kind())
	}
	return decodeFormStruct(rv, values)
}

func decodeFormStruct(rv reflect.Value, values url.Values) error {
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		name := field.Tag.Get("form")
		if name == "" {
			name = strings.TrimSuffix(field.Tag.Get("json"), ",omitempty")
		}
		if name == "" || name == "-" {
			continue
		}

		vals, ok := values[name]
		if !ok || len(vals) == 0 {
			continue
		}
		if err := setFormValue(rv.Field(i), vals); err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
	}
	return nil
}

func setFormValue(f reflect.Value, values []string) error {
	switch f.Kind() {
	case reflect.String:
		f.SetString(values[0])
	case reflect.Bool:
		v, err := strconv.ParseBool(values[0])
		if err != nil {
			return err
		}
		f.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(values[0], 10, f.Type().Bits())
		if err != nil {
			return err
		}
		f.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(values[0], 10, f.Type().Bits())
		if err != nil {
			return err
		}
		f.SetUint(v)
	case reflect.Slice:
		slice := reflect.MakeSlice(f.Type(), 0, len(values))
		for _, value := range values {
			item := reflect.New(f.Type().Elem()).Elem()
			if err := setFormValue(item, []string{value}); err != nil {
				return err
			}
			slice = reflect.Append(slice, item)
		}
		f.Set(slice)
	default:
		return fmt.Errorf("unsupported type %s", f.Type())
	}
	return nil
}
