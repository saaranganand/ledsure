package platform

import (
	"testing"
	"time"
)

func TestSystemClockNow(t *testing.T) {
	got := SystemClock{}.Now()

	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
	if !got.Equal(got.Truncate(time.Microsecond)) {
		t.Errorf("now = %v, want microsecond precision", got)
	}
	if d := time.Since(got); d < 0 || d > time.Second {
		t.Errorf("now is %v away from the real time", d)
	}
}
