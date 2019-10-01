package signature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// jsonFormatError is returned when JSON does not match expected format.
type jsonFormatError string

func (err jsonFormatError) Error() string {
	return string(err)
}

// paranoidUnmarshalJSONObject unmarshals data as a JSON object, but failing on the slightest unexpected aspect
// (including duplicated keys, unrecognized keys, and non-matching types). Uses fieldResolver to
// determine the destination for a field value, which should return a pointer to the destination if valid, or nil if the key is rejected.
//
// The fieldResolver approach is useful for decoding the Policy.Transports map; using it for structs is a bit lazy,
// we could use reflection to automate this. Later?
func paranoidUnmarshalJSONObject(data []byte, fieldResolver func(string) interface{}) error {
	seenKeys := map[string]struct{}{}

	dec := json.NewDecoder(bytes.NewReader(data))
	t, err := dec.Token()
	if err != nil {
		return jsonFormatError(err.Error())
	}
	if t != json.Delim('{') {
		return jsonFormatError(fmt.Sprintf("JSON object expected, got \"%s\"", t))
	}
	for {
		t, err := dec.Token()
		if err != nil {
			return jsonFormatError(err.Error())
		}
		if t == json.Delim('}') {
			break
		}

		key, ok := t.(string)
		if !ok {
			// Coverage: This should never happen, dec.Token() rejects non-string-literals in this state.
			return jsonFormatError(fmt.Sprintf("Key string literal expected, got \"%s\"", t))
		}
		if _, ok := seenKeys[key]; ok {
			return jsonFormatError(fmt.Sprintf("Duplicate key \"%s\"", key))
		}
		seenKeys[key] = struct{}{}

		valuePtr := fieldResolver(key)
		if valuePtr == nil {
			return jsonFormatError(fmt.Sprintf("Unknown key \"%s\"", key))
		}
		// This works like json.Unmarshal, in particular it allows us to implement UnmarshalJSON to implement strict parsing of the field value.
		if err := dec.Decode(valuePtr); err != nil {
			return jsonFormatError(err.Error())
		}
	}
	if _, err := dec.Token(); err != io.EOF {
		return jsonFormatError("Unexpected data after JSON object")
	}
	return nil
}

// paranoidUnmarshalJSONObject unmarshals data as a JSON object, but failing on the slightest unexpected aspect
// (including duplicated keys, unrecognized keys, and non-matching types). Each of the fields in exactFields
// must be present exactly once, and none other fields are accepted.
func paranoidUnmarshalJSONObjectExactFields(data []byte, exactFields map[string]interface{}) error {
	seenKeys := map[string]struct{}{}
	if err := paranoidUnmarshalJSONObject(data, func(key string) interface{} {
		if valuePtr, ok := exactFields[key]; ok {
			seenKeys[key] = struct{}{}
			return valuePtr
		}
		return nil
	}); err != nil {
		return err
	}
	for key := range exactFields {
		if _, ok := seenKeys[key]; !ok {
			return jsonFormatError(fmt.Sprintf(`Key "%s" missing in a JSON object`, key))
		}
	}
	return nil
}
