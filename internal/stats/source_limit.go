package stats

import (
	"math"
	"sync"
	"time"
)

type sourceBucket struct {
	tokens float64
	last   time.Time
}
type sourceLimits struct {
	mu      sync.Mutex
	buckets map[string]sourceBucket
	swept   time.Time
}

func (l *sourceLimits) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = map[string]sourceBucket{}
	}
	if now.Sub(l.swept) >= time.Minute {
		for key, bucket := range l.buckets {
			if now.Sub(bucket.last) >= 10*time.Minute {
				delete(l.buckets, key)
			}
		}
		l.swept = now
	}
	bucket, ok := l.buckets[ip]
	if !ok {
		if len(l.buckets) >= 4096 {
			return false
		}
		bucket = sourceBucket{tokens: 40, last: now}
	}
	if now.After(bucket.last) {
		bucket.tokens = math.Min(40, bucket.tokens+now.Sub(bucket.last).Seconds()*20)
		bucket.last = now
	}
	allowed := bucket.tokens >= 1
	if allowed {
		bucket.tokens--
	}
	l.buckets[ip] = bucket
	return allowed
}
