package platform

import "math/rand/v2"

type Rand interface {
	Int64N(n int64) int64
}

type SystemRand struct{}

func (SystemRand) Int64N(n int64) int64 {
	return rand.Int64N(n)
}

var _ Rand = SystemRand{}
