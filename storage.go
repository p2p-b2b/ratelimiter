package ratelimiter

// Storage interface defines the methods for a storage system
// Must be implemented by any storage system used in the rate limiter
type Storage interface {
	Store(key any, value any)
	Delete(key any)
	Load(key any) (value any, ok bool)
}
