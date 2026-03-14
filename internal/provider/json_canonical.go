package provider

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
)

// canonicalEncode encodes an arbitrary JSON-compatible value into a
// deterministic JSON string with lexicographically sorted object keys.
// Supported types are those returned by encoding/json when unmarshalling
// into interface{}: bool, float64, string, []interface{}, map[string]interface{}, nil.
func canonicalEncode(v interface{}) (string, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// CanonicalizeJSONFromString parses an input JSON string and returns its
// canonical (deterministically ordered) JSON representation.
func CanonicalizeJSONFromString(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	var v interface{}
	dec := json.NewDecoder(bytes.NewBufferString(s))
	// Use json.Number if numbers are large; we still normalize via encodeValue
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	return canonicalEncode(v)
}

// CanonicalizeJSONValue returns canonical JSON for an already parsed value
// (map[string]any, []any, primitives, etc.).
func CanonicalizeJSONValue(v interface{}) (string, error) { //nolint:revive // exported for reuse across resources
	return canonicalEncode(v)
}

// encodeValue writes canonical JSON into buf for supported Go value kinds.
func encodeValue(buf *bytes.Buffer, v interface{}) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case json.Number:
		// Validate the number and write it as-is (no quotes)
		if _, err := t.Int64(); err == nil {
			buf.WriteString(t.String())
			return nil
		}
		if _, err := t.Float64(); err == nil {
			buf.WriteString(t.String())
			return nil
		}
		// Fallback through json.Marshal for unusual number encodings
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case float64:
		// Use strconv/Marshal to avoid locale-specific formatting
		// json.Marshal on float64 is stable
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case string:
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case []interface{}:
		buf.WriteByte('[')
		for i, elem := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeValue(buf, elem); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case map[string]interface{}:
		// Sort keys for deterministic order
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			// Key
			buf.WriteByte('"')
			buf.WriteString(escapeString(k))
			buf.WriteByte('"')
			buf.WriteByte(':')
			// Value
			if err := encodeValue(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	default:
		// Fallback: attempt to re-marshal with encoding/json for supported structs
		// This also covers numbers represented as other numeric types (int, etc.)
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		// Ensure we still pass through canonicalization for any nested maps
		var v2 interface{}
		if err := json.Unmarshal(b, &v2); err != nil {
			return err
		}
		return encodeValue(buf, v2)
	}
}

// escapeString is a minimal JSON string escaper for keys; for values we rely on json.Marshal.
func escapeString(s string) string {
	// Use json.Marshal to escape, then strip surrounding quotes for re-use
	b, _ := json.Marshal(s)
	// b is like "..."
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1])
	}
	// Fallback: ensure any quotes/backslashes are escaped
	return strconv.QuoteToASCII(s)[1 : len(strconv.QuoteToASCII(s))-1]
}
