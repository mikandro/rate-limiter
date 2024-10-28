package ratelimit

import (
	"time"
)

type Limiter interface {
	Take() time.Time
}

type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}
