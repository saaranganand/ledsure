package platform

import (
	"time"
)

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond) // postgres timestamps have microsecond precision
}

var _ Clock = SystemClock{}
