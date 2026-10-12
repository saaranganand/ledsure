package platform

import "github.com/google/uuid"

type IDGenerator interface {
	New() uuid.UUID
}

// v7 since it starts with 48 bit timestamp in microseconds - helps with sorting
type UUIDv7Generator struct{}

func (UUIDv7Generator) New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

var _ IDGenerator = UUIDv7Generator{}
