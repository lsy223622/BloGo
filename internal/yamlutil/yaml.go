// Package yamlutil decodes the fixed YAML schemas used by configuration and content.
package yamlutil

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Decode rejects unknown fields, duplicate keys, scalar coercions and extra documents.
// Nodes is returned so callers can attach field locations to semantic errors.
func Decode(data []byte, out any) (*yaml.Node, error) {
	d := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := d.Decode(&doc); err != nil {
		return nil, err
	}
	var next yaml.Node
	if err := d.Decode(&next); err != io.EOF {
		return nil, fmt.Errorf("expected one YAML document")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping")
	}
	n := doc.Content[0]
	if err := plain(n, map[*yaml.Node]bool{}); err != nil {
		return nil, err
	}
	if err := shape(n, reflect.TypeOf(out).Elem(), ""); err != nil {
		return nil, err
	}
	if err := n.Decode(out); err != nil {
		return nil, err
	}
	return n, nil
}

func Error(n *yaml.Node, field, message string) error {
	return fmt.Errorf("line %d:%d: %s: %s", n.Line, n.Column, field, message)
}

func Field(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return n
}

func plain(n *yaml.Node, stack map[*yaml.Node]bool) error {
	if stack[n] {
		return Error(n, "YAML", "recursive alias")
	}
	stack[n] = true
	defer delete(stack, n)
	if n.Kind == yaml.AliasNode {
		return plain(n.Alias, stack)
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag != "!!str" || k.Kind != yaml.ScalarNode {
				return Error(k, "YAML", "mapping keys must be strings")
			}
			if seen[k.Value] {
				return Error(k, k.Value, "duplicate key")
			}
			seen[k.Value] = true
		}
	}
	for _, child := range n.Content {
		if err := plain(child, stack); err != nil {
			return err
		}
	}
	if n.Kind == yaml.ScalarNode {
		switch n.Tag {
		case "!!str", "!!bool", "!!int", "!!float", "!!null", "!!timestamp":
		default:
			return Error(n, "YAML", "unsupported scalar tag")
		}
	}
	return nil
}

var unmarshaler = reflect.TypeFor[yaml.Unmarshaler]()

func shape(n *yaml.Node, typ reflect.Type, field string) error {
	if n.Kind == yaml.AliasNode {
		return shape(n.Alias, typ, field)
	}
	if typ.Kind() == reflect.Pointer {
		return shape(n, typ.Elem(), field)
	}
	if typ.Kind() == reflect.Interface {
		return nil
	}
	if n.Tag == "!!null" {
		return Error(n, field, "null is not allowed")
	}
	if reflect.PointerTo(typ).Implements(unmarshaler) {
		return nil
	}
	valid := true
	switch typ.Kind() {
	case reflect.Struct:
		valid = n.Kind == yaml.MappingNode
		if valid {
			fields := map[string]reflect.Type{}
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				name := strings.Split(f.Tag.Get("yaml"), ",")[0]
				if name != "" && name != "-" {
					fields[name] = f.Type
				}
			}
			for i := 0; i < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				ft, ok := fields[k.Value]
				if !ok {
					return Error(k, k.Value, "unknown field")
				}
				path := k.Value
				if field != "" {
					path = field + "." + path
				}
				if err := shape(v, ft, path); err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		valid = n.Kind == yaml.MappingNode
		if valid {
			for i := 0; i < len(n.Content); i += 2 {
				if err := shape(n.Content[i+1], typ.Elem(), field+"."+n.Content[i].Value); err != nil {
					return err
				}
			}
		}
	case reflect.Slice:
		valid = n.Kind == yaml.SequenceNode
		if valid {
			for _, v := range n.Content {
				if err := shape(v, typ.Elem(), field); err != nil {
					return err
				}
			}
		}
	case reflect.String:
		valid = n.Tag == "!!str"
	case reflect.Bool:
		valid = n.Tag == "!!bool"
	case reflect.Int, reflect.Int64:
		valid = n.Tag == "!!int"
	default:
		return Error(n, field, "unsupported schema type")
	}
	if !valid {
		return Error(n, field, "expected "+typ.Kind().String())
	}
	return nil
}
