package form

import "encoding/json"

// encode and decode copy a list for undo (EditableList.svelte keeps JSON
// snapshots as well).
func encode[T any](v []T) []byte {
	b, _ := json.Marshal(v)
	return b
}

func decode[T any](b []byte, v *[]T) {
	var out []T
	if json.Unmarshal(b, &out) == nil {
		*v = out
	}
}
