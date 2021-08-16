package rate_limit

import (
	"time"
)

type RateLimit struct {
	Enable bool
	Bucket *chan struct{}
}

func (rl *RateLimit) GetBucket(rate float64, max int64) chan struct{} {
	rl.Bucket = new(chan struct{})
	if rate == 0 || max == 0 {
		return *rl.Bucket
	}
	rl.Enable = true
	*rl.Bucket = make(chan struct{}, max)
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	go func() {
		for {
			select {
			case <-ticker.C:
				select {
				case *rl.Bucket <- struct{}{}:
				default:
				}
			}
		}
	}()
	return *rl.Bucket
}

func (rl *RateLimit) GetToken(block bool) bool {
	if !rl.Enable {
		return true
	} else if block {
		select {
		case <-*rl.Bucket:
			return true
		}
	} else {
		select {
		case <-*rl.Bucket:
			return true
		default:
			return false
		}
	}
}
