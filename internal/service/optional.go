package service

import (
	"bytes"
	"encoding/json"
)

// Opt is a PATCH field: absent (Set=false), null (Set=true, Null=true) or a
// value. It lets callers clear a field by sending null.
type Opt[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON is only called when the field is present.
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// Some returns a set, non-null Opt.
func Some[T any](v T) Opt[T] { return Opt[T]{Set: true, Value: v} }

// Null returns a set, null Opt.
func Null[T any]() Opt[T] { return Opt[T]{Set: true, Null: true} }
