// =============================================================================
// HYDRA-UMC-TOOL-CLI - A small JSON Schema checker: cmd/hydra-cli/jsonschema.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// The schemas of the HYDRA-UMC SDK use a small part of JSON Schema: type, const, enum, required, properties,
// additionalProperties (true or false), pattern, minLength, minItems, uniqueItems, items, minimum, maximum and
// oneOf. This file checks a document against a schema that uses only those. A schema that uses any other
// keyword that restricts a document is refused with an error instead of being checked in part, so a check
// never claims more than it did.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// annotationKeywords say something about the schema without restricting a document.
var annotationKeywords = map[string]bool{
	"$schema": true, "$id": true, "title": true, "description": true, "examples": true, "default": true, "$comment": true,
}

// restrictingKeywords are the ones this checker understands.
var restrictingKeywords = map[string]bool{
	"type": true, "const": true, "enum": true, "required": true, "properties": true, "additionalProperties": true,
	"pattern": true, "minLength": true, "maxLength": true, "minItems": true, "maxItems": true, "uniqueItems": true,
	"items": true, "minimum": true, "maximum": true, "oneOf": true,
}

// schemaViolation is one place where a document does not fit its schema.
type schemaViolation struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (v schemaViolation) String() string { return v.Path + ": " + v.Message }

// validateAgainstSchema checks a JSON document (already decoded with encoding/json) against a decoded schema.
func validateAgainstSchema(schema map[string]any, document any) ([]schemaViolation, error) {
	var violations []schemaViolation
	if err := checkNode(schema, document, "$", &violations); err != nil {
		return nil, err
	}
	sort.SliceStable(violations, func(a, b int) bool { return violations[a].Path < violations[b].Path })
	return violations, nil
}

func jsonType(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case float64:
		if v == math.Trunc(v) {
			return "integer"
		}
		return "number"
	}
	return "unknown"
}

func typeMatches(want string, value any) bool {
	got := jsonType(value)
	return got == want || (want == "number" && got == "integer")
}

func checkNode(schema map[string]any, value any, path string, out *[]schemaViolation) error {
	for key := range schema {
		if !annotationKeywords[key] && !restrictingKeywords[key] {
			return fmt.Errorf("the schema uses the keyword %q at %s, which this checker does not understand", key, path)
		}
	}
	fail := func(format string, args ...any) {
		*out = append(*out, schemaViolation{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	if raw, ok := schema["type"]; ok {
		var wanted []string
		switch t := raw.(type) {
		case string:
			wanted = []string{t}
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok {
					wanted = append(wanted, s)
				}
			}
		}
		matched := false
		for _, want := range wanted {
			if typeMatches(want, value) {
				matched = true
			}
		}
		if !matched {
			fail("must be %s, found %s", strings.Join(wanted, " or "), jsonType(value))
			return nil
		}
	}
	if want, ok := schema["const"]; ok && !reflect.DeepEqual(want, value) {
		fail("must be %v", want)
	}
	if raw, ok := schema["enum"].([]any); ok {
		found := false
		options := make([]string, 0, len(raw))
		for _, option := range raw {
			options = append(options, fmt.Sprint(option))
			if reflect.DeepEqual(option, value) {
				found = true
			}
		}
		if !found {
			fail("must be one of %s", strings.Join(options, ", "))
		}
	}
	if text, ok := value.(string); ok {
		if raw, ok := schema["pattern"].(string); ok {
			re, err := regexp.Compile(raw)
			if err != nil {
				return fmt.Errorf("the pattern %q at %s is not a regular expression this checker reads: %w", raw, path, err)
			}
			if !re.MatchString(text) {
				fail("%q does not match %s", text, raw)
			}
		}
		if n, ok := schema["minLength"].(float64); ok && float64(len([]rune(text))) < n {
			fail("must have at least %d character(s)", int(n))
		}
		if n, ok := schema["maxLength"].(float64); ok && float64(len([]rune(text))) > n {
			fail("must have at most %d character(s)", int(n))
		}
	}
	if number, ok := value.(float64); ok {
		if n, ok := schema["minimum"].(float64); ok && number < n {
			fail("must be at least %v", n)
		}
		if n, ok := schema["maximum"].(float64); ok && number > n {
			fail("must be at most %v", n)
		}
	}
	if items, ok := value.([]any); ok {
		if n, ok := schema["minItems"].(float64); ok && float64(len(items)) < n {
			fail("must have at least %d item(s)", int(n))
		}
		if n, ok := schema["maxItems"].(float64); ok && float64(len(items)) > n {
			fail("must have at most %d item(s)", int(n))
		}
		if unique, ok := schema["uniqueItems"].(bool); ok && unique {
			for i := range items {
				for j := i + 1; j < len(items); j++ {
					if reflect.DeepEqual(items[i], items[j]) {
						fail("items %d and %d are the same", i, j)
					}
				}
			}
		}
		if sub, ok := schema["items"].(map[string]any); ok {
			for i, item := range items {
				if err := checkNode(sub, item, fmt.Sprintf("%s[%d]", path, i), out); err != nil {
					return err
				}
			}
		}
	}
	if object, ok := value.(map[string]any); ok {
		if required, ok := schema["required"].([]any); ok {
			for _, name := range required {
				if key, ok := name.(string); ok {
					if _, present := object[key]; !present {
						fail("is missing the required field %q", key)
					}
				}
			}
		}
		properties, _ := schema["properties"].(map[string]any)
		names := make([]string, 0, len(object))
		for name := range object {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			sub, known := properties[name].(map[string]any)
			if known {
				if err := checkNode(sub, object[name], path+"."+name, out); err != nil {
					return err
				}
				continue
			}
			switch extra := schema["additionalProperties"].(type) {
			case bool:
				if !extra {
					*out = append(*out, schemaViolation{Path: path + "." + name, Message: "is not a field this schema allows"})
				}
			case map[string]any:
				if err := checkNode(extra, object[name], path+"."+name, out); err != nil {
					return err
				}
			}
		}
	}
	if options, ok := schema["oneOf"].([]any); ok {
		matches := 0
		for _, option := range options {
			sub, ok := option.(map[string]any)
			if !ok {
				continue
			}
			var inner []schemaViolation
			if err := checkNode(sub, value, path, &inner); err != nil {
				return err
			}
			if len(inner) == 0 {
				matches++
			}
		}
		if matches != 1 {
			fail("must match exactly one of %d alternatives, matches %d", len(options), matches)
		}
	}
	return nil
}

// decodeJSON decodes a document, refusing trailing content and keeping numbers as float64.
func decodeJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("there is more than one JSON value")
	}
	return value, nil
}
