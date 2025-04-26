// The rate limiter is a token bucket algorithm
// https://en.wikipedia.org/wiki/Token_bucket
package ratelimiter

import (
	"context"
	"time"
)

// BucketLimiter is a rate limiter that uses a bucket algorithm
// It allows a certain number of requests in a given time frame
// and then limits the requests to a certain rate
// It uses a storage system to store the state of the limiter
// and a deleteAfter duration to remove the limiter after a certain time
// It implements the Limiter interface
// and uses the Storage interface to store the state of the limiter
type BucketLimiter struct {
	deleteAfter time.Duration
	storage     Storage
	limiter     Limiter
}

// NewBucketLimiter creates a new BucketLimiter
// with the given limiter, deleteAfter duration and storage system
func NewBucketLimiter(limiter Limiter, deleteAfter time.Duration, storage Storage) *BucketLimiter {
	return &BucketLimiter{
		deleteAfter: deleteAfter,
		storage:     storage,
		limiter:     limiter,
	}
}

// GetOrAdd gets the limiter for the given key from the storage
// If the limiter does not exist, it creates a new one
// and stores it in the storage
// It returns the limiter for the given key
// It also starts a goroutine to delete the limiter after the deleteAfter duration
func (b *BucketLimiter) GetOrAdd(key string) (value Limiter) {
	limiter, ok := b.storage.Load(key)
	if !ok {
		limiter = NewBucketLimiter(b.limiter, b.deleteAfter, b.storage)
		b.storage.Store(key, limiter)

		go func() {
			time.Sleep(b.deleteAfter)
			b.storage.Delete(key)
		}()
	}

	return limiter.(Limiter)
}

// Remove removes the limiter for the given key from the storage
func (b *BucketLimiter) Remove(key string) error {
	b.storage.Delete(key)

	return nil
}

// Wait waits for the limiter to allow the request
// It returns an error if the request is not allowed
// It uses the Wait method of the limiter
func (b *BucketLimiter) Wait(ctx context.Context) error {
	return b.limiter.Wait(ctx)
}

// Allow checks if the request is allowed by the limiter
// It returns true if the request is allowed, false otherwise
func (b *BucketLimiter) Allow() bool {
	return b.limiter.Allow()
}

// Burst returns the burst size of the limiter
// It returns the burst size of the limiter
func (b *BucketLimiter) Burst() int {
	return b.limiter.Burst()
}
