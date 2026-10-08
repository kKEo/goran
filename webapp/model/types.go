package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// JSON stores an arbitrary JSON document in a text column and renders it
// verbatim in API responses.
type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "null", nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case string:
		*j = JSON(v)
	case []byte:
		*j = JSON(append([]byte(nil), v...))
	default:
		return fmt.Errorf("cannot scan %T into model.JSON", value)
	}
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return fmt.Errorf("model.JSON: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[:0], data...)
	return nil
}

// Labels is a sorted set of plain strings stored as a comma separated list.
// Tasks use them to require agents with matching labels (for example a
// client's network or region).
type Labels []string

// NormalizeLabels trims, drops empties, dedupes and sorts.
func NormalizeLabels(in []string) Labels {
	seen := map[string]bool{}
	out := Labels{}
	for _, l := range in {
		l = strings.TrimSpace(l)
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

func (l Labels) Value() (driver.Value, error) {
	return strings.Join(l, ","), nil
}

func (l *Labels) Scan(value interface{}) error {
	var s string
	switch v := value.(type) {
	case nil:
		*l = Labels{}
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("cannot scan %T into model.Labels", value)
	}
	*l = NormalizeLabels(strings.Split(s, ","))
	return nil
}

// ContainsAll reports whether every required label is present.
func (l Labels) ContainsAll(required Labels) bool {
	have := map[string]bool{}
	for _, x := range l {
		have[x] = true
	}
	for _, r := range required {
		if !have[r] {
			return false
		}
	}
	return true
}

// StringMap stores a map[string]string as JSON text.
type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (m *StringMap) Scan(value interface{}) error {
	var b []byte
	switch v := value.(type) {
	case nil:
		*m = StringMap{}
		return nil
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		return fmt.Errorf("cannot scan %T into model.StringMap", value)
	}
	if len(b) == 0 {
		*m = StringMap{}
		return nil
	}
	return json.Unmarshal(b, (*map[string]string)(m))
}
