package ratelimiter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// Helper function to create a base limiter for tests
func newTestBaseLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Limit(10), 5) // 10 tokens/sec, burst 5
}

// request simulates a request being processed
func request(t testing.TB, who string) {
	t.Helper()

	time.Sleep(500 * time.Millisecond)
	_, err := fmt.Printf("Request from %s done\n", who)
	if err != nil {
		t.Errorf("Failed to print message: %v", err)
	}
}

// TestNewBucketLimiter tests the constructor
func TestNewBucketLimiter(t *testing.T) {
	storage := NewInMemoryStorage()
	baseLimiter := newTestBaseLimiter()
	deleteAfter := 5 * time.Second

	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage) // Pass pointer to storage

	assert.NotNil(t, bl, "BucketLimiter should not be nil")
	assert.Equal(t, deleteAfter, bl.deleteAfter, "deleteAfter duration should match")
	assert.Same(t, storage, bl.storage, "Storage should match") // Use assert.Same for pointer identity
	assert.Equal(t, baseLimiter, bl.limiter, "Base limiter should match")
}

// TestBucketLimiter_GetOrAdd_New tests adding a new limiter
func TestBucketLimiter_GetOrAdd_New(t *testing.T) {
	storage := NewInMemoryStorage()
	baseLimiter := newTestBaseLimiter()
	deleteAfter := 100 * time.Millisecond // Short duration for testing cleanup
	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	key := "test-key-new"

	// First call should create and return a new limiter
	limiter1 := bl.GetOrAdd(key)
	assert.NotNil(t, limiter1, "Returned limiter should not be nil")

	// Check if it's stored
	storedLimiter, ok := storage.Load(key)
	assert.True(t, ok, "Limiter should be stored")
	assert.Equal(t, limiter1, storedLimiter, "Returned limiter should be the one stored")
}

// TestBucketLimiter_GetOrAdd_Existing tests retrieving an existing limiter
func TestBucketLimiter_GetOrAdd_Existing(t *testing.T) {
	storage := NewInMemoryStorage()
	baseLimiter := newTestBaseLimiter()
	deleteAfter := 5 * time.Second
	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	key := "test-key-existing"

	// Add it first
	limiter1 := bl.GetOrAdd(key)
	assert.NotNil(t, limiter1)

	// Get it again
	limiter2 := bl.GetOrAdd(key)
	assert.NotNil(t, limiter2)
	assert.Same(t, limiter1, limiter2, "Getting an existing key should return the same limiter instance")
}

// TestBucketLimiter_GetOrAdd_DeleteAfter tests automatic deletion
func TestBucketLimiter_GetOrAdd_DeleteAfter(t *testing.T) {
	storage := NewInMemoryStorage()
	baseLimiter := newTestBaseLimiter()
	deleteAfter := 50 * time.Millisecond // Very short duration
	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	key := "test-key-delete"

	// Add the limiter
	limiter1 := bl.GetOrAdd(key)
	assert.NotNil(t, limiter1)

	// Check it's there
	_, ok := storage.Load(key)
	assert.True(t, ok, "Limiter should be present initially")

	// Wait longer than deleteAfter
	time.Sleep(deleteAfter + 20*time.Millisecond)

	// Check if it's deleted
	_, ok = storage.Load(key)
	assert.False(t, ok, "Limiter should be deleted after deleteAfter duration")

	// Getting it again should create a new one
	limiter2 := bl.GetOrAdd(key)
	assert.NotNil(t, limiter2, "Should get a new limiter after deletion")
	assert.NotSame(t, limiter1, limiter2, "Should be a different instance after deletion and re-creation")

	// Check the new one is stored
	_, ok = storage.Load(key)
	assert.True(t, ok, "New limiter should be stored after re-creation")
}

// TestBucketLimiter_Remove tests manual removal
func TestBucketLimiter_Remove(t *testing.T) {
	storage := NewInMemoryStorage()
	baseLimiter := newTestBaseLimiter()
	deleteAfter := 5 * time.Second
	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	key := "test-key-remove"

	// Add it
	bl.GetOrAdd(key)
	_, ok := storage.Load(key)
	assert.True(t, ok, "Limiter should be present before removal")

	// Remove it
	err := bl.Remove(key)
	assert.NoError(t, err, "Remove should not return an error")

	// Check it's gone
	_, ok = storage.Load(key)
	assert.False(t, ok, "Limiter should be absent after removal")
}

// TestBucketLimiter_GetOrAdd_Concurrent tests concurrent access
func TestBucketLimiter_GetOrAdd_Concurrent(t *testing.T) {
	storage := NewInMemoryStorage()
	baseLimiter := newTestBaseLimiter()
	deleteAfter := 200 * time.Millisecond
	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	key := "concurrent-key"
	numGoroutines := 50
	var wg sync.WaitGroup
	var firstLimiter Limiter
	var mu sync.Mutex // Protect access to firstLimiter

	wg.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer wg.Done()
			limiter := bl.GetOrAdd(key)
			assert.NotNil(t, limiter)

			mu.Lock()
			if firstLimiter == nil {
				firstLimiter = limiter
			} else {
				// All subsequent calls should get the same instance until it's potentially deleted
				assert.Same(t, firstLimiter, limiter, "Concurrent GetOrAdd should return the same instance")
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Verify it's still stored (or was stored at some point)
	storedLimiter, ok := storage.Load(key)
	assert.True(t, ok, "Limiter should be stored after concurrent access")
	assert.Same(t, firstLimiter, storedLimiter, "Stored limiter should be the one returned by goroutines")

	// Test deletion after concurrent access
	time.Sleep(deleteAfter + 50*time.Millisecond)
	_, ok = storage.Load(key)
	assert.False(t, ok, "Limiter should be deleted after deleteAfter duration following concurrent access")
}

func TestBucketLimiter_ConcurrentAccess_One_Target(t *testing.T) {
	limit := 5.0
	burst := 5
	deleteAfter := time.Second * 5

	storage := NewInMemoryStorage()
	baseLimiter := rate.NewLimiter(rate.Limit(limit), burst)

	limiter := NewBucketLimiter(baseLimiter, deleteAfter, storage)
	numRequests := 100
	rateSeg := time.Second * 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(numRequests)

	go func() {
		ticker := time.NewTicker(rateSeg)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for i := range numRequests {
					go func(who string, reqNum int) {
						defer wg.Done()

						lim := limiter.GetOrAdd(who)
						if !lim.Allow() {
							_, err := fmt.Printf("Request from %s is limited\n", who)
							if err != nil {
								t.Errorf("Failed to print message: %v", err)
							}

							return
						}

						request(t, who)
					}("anonymous", i)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

func TestBucketLimiter_ConcurrentAccess_Multiple_Targets(t *testing.T) {
	limit := 5.0
	burst := 5

	deleteAfter := time.Second * 5
	storage := NewInMemoryStorage()
	baseLimiter := rate.NewLimiter(rate.Limit(limit), burst)

	limiter := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	numRequests := 100
	numTargets := 2

	rateSeg := time.Second * 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(numRequests * numTargets)

	go func() {
		ticker := time.NewTicker(rateSeg)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for j := range numTargets {
					for i := range numRequests {
						go func(who string, reqNum int) {
							defer wg.Done()

							lim := limiter.GetOrAdd(who)
							if !lim.Allow() {
								_, err := fmt.Printf("Request from %s is limited\n", who)
								if err != nil {
									t.Errorf("Failed to print message: %v", err)
								}

								return
							}

							request(t, who)
						}(fmt.Sprintf("anonymous-%d", j), i)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

// Example_bucketLimiter demonstrates basic usage of the BucketLimiter.
func ExampleBucketLimiter() {
	// Create a storage (in-memory for this example)
	storage := NewInMemoryStorage()

	// Define the base rate limit: 5 requests per second with a burst of 3
	baseLimiter := rate.NewLimiter(rate.Limit(5), 3)

	// Create the BucketLimiter: limiters expire after 1 minute of inactivity
	deleteAfter := 1 * time.Minute
	bucketLimiter := NewBucketLimiter(baseLimiter, deleteAfter, storage)

	// Simulate requests for different keys (e.g., user IDs, IP addresses)
	keys := []string{"user-1", "user-2", "user-1", "user-3", "user-1", "user-2"}
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(len(keys))

	fmt.Println("Simulating requests...")

	for i, key := range keys {
		go func(k string, reqNum int) {
			defer wg.Done()

			// Get the specific limiter for this key
			lim := bucketLimiter.GetOrAdd(k)

			// Wait for permission according to the limit
			err := lim.Wait(ctx)
			if err != nil {
				fmt.Printf("Request %d (%s): Error waiting for limiter: %v\n", reqNum+1, k, err)
				return
			}

			// Process the request (simulated)
			fmt.Printf("Request %d (%s): Processed\n", reqNum+1, k)
			time.Sleep(50 * time.Millisecond) // Simulate work
		}(key, i)
		time.Sleep(100 * time.Millisecond) // Stagger requests slightly
	}

	wg.Wait()
	fmt.Println("All simulations finished.")

	// You can manually remove a limiter if needed
	_ = bucketLimiter.Remove("user-3")
	fmt.Println("Removed limiter for user-3.")

	// Limiters for user-1 and user-2 will be automatically removed
	// by the background cleanup goroutine after 'deleteAfter' duration
	// if they are not accessed again.

	// Output:
	// Simulating requests...
	// Request 1 (user-1): Processed
	// Request 2 (user-2): Processed
	// Request 3 (user-1): Processed
	// Request 4 (user-3): Processed
	// Request 5 (user-1): Processed
	// Request 6 (user-2): Processed
	// All simulations finished.
	// Removed limiter for user-3.
}

// TestBucketLimiter_Wait demonstrates the Wait method behavior.
func TestBucketLimiter_Wait(t *testing.T) {
	storage := NewInMemoryStorage()
	// Low limit for easier testing: 2 tokens/sec, burst 1
	baseLimiter := rate.NewLimiter(rate.Limit(2), 1)
	// Increase deleteAfter to be longer than the expected test duration (~1.5s)
	// to prevent premature cleanup during the Wait calls.
	deleteAfter := 3 * time.Second // Increased from 1 second
	bl := NewBucketLimiter(baseLimiter, deleteAfter, storage)
	key := "test-wait-key"
	ctx := context.Background()

	limiter := bl.GetOrAdd(key)

	numRequests := 4
	var wg sync.WaitGroup
	wg.Add(numRequests)
	startTime := time.Now()

	for i := range numRequests {
		go func(reqNum int) {
			defer wg.Done()
			err := limiter.Wait(ctx)
			assert.NoError(t, err, "Wait should not return an error in this test")
			// Use t.Logf for logging within tests
			t.Logf("Request %d processed at %v", reqNum, time.Since(startTime))
		}(i)
		// Add a small delay to stagger goroutine starts slightly,
		// making timing less dependent on scheduler specifics.
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	// Expected time:
	// Req 0: ~0s (burst token)
	// Req 1: ~0.5s (waits for token regeneration)
	// Req 2: ~1.0s
	// Req 3: ~1.5s
	// Total time should be around 1.5 seconds for 4 requests at 2 req/sec with burst 1.
	// Allow some buffer for scheduling delays.
	minExpectedDuration := 1350 * time.Millisecond // Slightly less than 1.5s
	maxExpectedDuration := 2100 * time.Millisecond // Allow more buffer

	assert.GreaterOrEqual(t, elapsed, minExpectedDuration, "Elapsed time should be at least %v", minExpectedDuration)
	assert.LessOrEqual(t, elapsed, maxExpectedDuration, "Elapsed time should be at most %v", maxExpectedDuration)

	// Check cleanup works after waiting
	// This check should now pass because deleteAfter (3s) > elapsed (~1.5s)
	_, ok := storage.Load(key)
	assert.True(t, ok, "Limiter should still be present immediately after Wait calls")

	// Now wait for the actual deleteAfter duration to pass
	time.Sleep(deleteAfter + 100*time.Millisecond)
	_, ok = storage.Load(key)
	assert.False(t, ok, "Limiter should be deleted after deleteAfter duration")
}
