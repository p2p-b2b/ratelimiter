package ratelimiter

import "context"

// Limiter interface defines the methods for a rate limiter
// Must be implemented by any rate limiting algorithm
type Limiter interface {
	Burst() int
	Allow() bool
	Wait(ctx context.Context) (err error)
}
