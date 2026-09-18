package provider

import (
	"bytes"
	"encoding/json"
)

// projectJSON narrows server to the keys present in mask, recursing into
// nested objects.
//
// Graylog materializes the defaults of a lookup cache or data adapter
// configuration when it stores one: submitting five keys to a guava_cache can
// read back nine, the extra four being nulls the practitioner never wrote.
// Comparing the full echo against the configuration would therefore report a
// difference on every plan, so only the keys actually under management take
// part in drift detection.
//
// Array membership is compared whole: an element added or removed server-side
// is a real change. Arrays of equal length are projected element-wise so
// defaults added inside object elements are ignored without hiding membership
// drift.
func projectJSON(server, mask any) any {
	maskArray, maskIsArray := mask.([]any)
	serverArray, serverIsArray := server.([]any)
	if maskIsArray && serverIsArray {
		if len(maskArray) != len(serverArray) {
			return server
		}
		out := make([]any, len(serverArray))
		for i := range serverArray {
			out[i] = projectJSON(serverArray[i], maskArray[i])
		}
		return out
	}

	maskObj, ok := mask.(map[string]any)
	if !ok {
		return server
	}
	serverObj, ok := server.(map[string]any)
	if !ok {
		return server
	}

	out := make(map[string]any, len(maskObj))
	for key, maskValue := range maskObj {
		serverValue, present := serverObj[key]
		if !present {
			// A managed key that the server dropped is itself the drift; its
			// absence from the projection surfaces as a difference.
			continue
		}
		out[key] = projectJSON(serverValue, maskValue)
	}
	return out
}

// decodeJSONPreservingNumbers decodes into interface{} while keeping numbers
// as json.Number. Decoding them as float64 would re-encode large integers in
// scientific notation (20000000 becomes 2e+07) and manufacture a difference
// out of nothing.
func decodeJSONPreservingNumbers(raw string) (any, error) {
	if raw == "" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewBufferString(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// ProjectAndCanonicalizeJSON returns the canonical JSON of serverJSON reduced
// to the keys present in maskJSON. An unparseable mask yields the canonical
// server document untouched, which is the right fallback for an imported
// resource whose state does not carry a document yet.
func ProjectAndCanonicalizeJSON(serverJSON, maskJSON string) (string, error) {
	server, err := decodeJSONPreservingNumbers(serverJSON)
	if err != nil {
		return "", err
	}
	mask, err := decodeJSONPreservingNumbers(maskJSON)
	if err != nil || mask == nil {
		return CanonicalizeJSONValue(server)
	}
	return CanonicalizeJSONValue(projectJSON(server, mask))
}
