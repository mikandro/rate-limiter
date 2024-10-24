package ratelimit

import (
	"time"
)

type Limiter interface {
	Take() time.Time
}
