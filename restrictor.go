package restrictor

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// Restrictor holds general information of limits
type Restrictor struct {
	window     uint32 // seconds
	upperLimit uint32
	bucketSpan uint32 // the time span of each bucket, seconds

	prefix string // key prefix, for separating limitors of different restrictor
	store  Store
}

// LimitReached check whether limit has been reached now
func (r *Restrictor) LimitReached(key string) (bool, error) {
	return r.LimitReachedAtTime(time.Now(), key)
}

// LimitReachedWithCount check whether limit has been reached now
// and returns current count for key within time window.
func (r *Restrictor) LimitReachedWithCount(key string) (bool, uint32, error) {
	return r.LimitReachedAtTimeWithCount(time.Now(), key)
}

// LimitReachedAtTime check whether limit has been reached at time 'now'
func (r *Restrictor) LimitReachedAtTime(now time.Time, key string) (bool, error) {
	reached, _, err := r.LimitReachedAtTimeWithCount(now, key)
	return reached, err
}

// LimitReachedAtTimeWithCount check whether limit has been reached at time 'now'
func (r *Restrictor) LimitReachedAtTimeWithCount(now time.Time, key string) (bool, uint32, error) {
	randMark := strconv.Itoa(time.Now().Nanosecond())
	// can not preceed further, return true
	if ok, err := r.store.TryLock(key, randMark); !ok || err != nil {
		return true, 0, err
	}

	lmt, expireTime, found := r.store.GetLimiter(r.prefix + key)
	if !found {
		lmt = NewLimiter()
	}
	reached, currentCount, lmtChanged, expireChanged := lmt.LimitReached(
		r.window,
		r.upperLimit, r.bucketSpan, now,
	)
	if lmtChanged {
		if expireChanged {
			r.store.SetLimiter(r.prefix+key, lmt, int(r.window))
		} else {
			r.store.SetLimiter(
				r.prefix+key, lmt,
				int(expireTime.Sub(time.Now()).Seconds()),
			)
		}
	}

	r.store.Unlock(key, randMark)
	return reached, currentCount, nil
}

// GetCount returns number of records 'now' within defined time window.
// Does not modify data (read only).
func (r *Restrictor) GetCount(key string, window time.Duration) (count uint32, err error) {
	if uint32(window.Seconds()) > r.window {
		return 0, fmt.Errorf("window value can't be bigger than restrictor window")
	}
	randMark := strconv.Itoa(time.Now().Nanosecond())
	// can not preceed further, return true
	if ok, e := r.store.TryLock(key, randMark); !ok || e != nil {
		return 0, err
	}

	lmt, _, found := r.store.GetLimiter(r.prefix + key)
	if found {
		count = lmt.GetCount(uint32(window.Seconds()), time.Now())
	}

	r.store.Unlock(key, randMark)
	return count, nil
}

// NewRestrictor creates a restrictor
// window should not be too large, it will be converted to 'seconds'
// limit is the max number of requests allowed in a window
// numberOfBuckets is number of buckets in the sliding window, usually around 100
func NewRestrictor(window time.Duration, limit, numberOfBuckets uint32,
	store Store) Restrictor {
	windowSec := uint32(window.Seconds())
	span := windowSec / numberOfBuckets
	if windowSec%numberOfBuckets > 0 {
		span++
	}

	return Restrictor{
		window:     windowSec,
		upperLimit: uint32(limit),
		bucketSpan: span,
		prefix:     fmt.Sprintf("%d_%02d_", time.Now().UnixNano(), rand.Intn(100)),
		store:      store,
	}
}
