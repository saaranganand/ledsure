package platform

import (
	"bytes"
	"testing"
)

func TestUUIDv7GeneratorNew(t *testing.T) {
	gen := UUIDv7Generator{}

	prev := gen.New()
	for range 1000 {
		id := gen.New()
		if id.Version() != 7 {
			t.Fatalf("version = %d, want 7", id.Version())
		}
		if bytes.Compare(prev[:], id[:]) >= 0 {
			t.Fatalf("ids not increasing: %s then %s", prev, id)
		}
		prev = id
	}
}
