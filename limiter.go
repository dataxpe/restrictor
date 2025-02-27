package restrictor

//go:generate protoc -I=. -I=$GOPATH/src -I=$GOPATH/src/github.com/gogo/protobuf/protobuf --gogoslick_out=. limiter.proto

import (
	"math"
	"time"
)

// NewLimiter creates a limiter
func NewLimiter() *Limiter {
	return &Limiter{FullUntil: 0, Buckets: make(map[uint32]uint32)}
}

// LimitReached checks whether limits reached
// side effect: the limiter itself is modified if limit is not reached yet
func (lmt *Limiter) LimitReached(window, upperLimit, interval uint32,
	now time.Time) (reached bool, count uint32, lmtChanged bool, expireChanged bool) {
	if upperLimit == 0 {
		return true, 0, false, false
	}

	ts := uint32(now.Unix())
	if ts < lmt.FullUntil { // total == upperLimit
		return true, upperLimit, false, false
	}

	boundary := ts - window

	total := uint32(0)
	oldest := uint32(math.MaxUint32)
	// remove old useless buckets outside window
	for t, count := range lmt.Buckets {
		if t <= boundary {
			delete(lmt.Buckets, t)
		} else {
			if oldest > t {
				oldest = t
			}
			total += count
		}
	}

	// limit not reached yet
	if total < upperLimit {
		// reset 'FullUntil'
		lmt.FullUntil = 0
		// normalized timestamp, because we only use a limited number of buckets
		normalizedTS := ts - (ts % interval)
		// update bucket count
		lmt.Buckets[normalizedTS]++
		return false, total + 1, true, true
	}

	// blcoked until 'FullUntil'
	lmt.FullUntil = oldest + window
	return true, upperLimit, true, false
}

// GetCount returns count of objects in bucket within defined time window.
// Does not modify data (read only).
func (lmt *Limiter) GetCount(window uint32, now time.Time) (total uint32) {
	ts := uint32(now.Unix())

	boundary := ts - window
	for t, count := range lmt.Buckets {
		if t > boundary {
			total += count
		}
	}
	return
}
