package rate_limit

import "time"

type RateLimit struct {
	bucket *chan struct{}
}

func (rl *RateLimit) GetBucket(rate float32, max int) chan struct{} {
	*rl.bucket = make(chan struct{}, max)
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	go func() {
		for {
			select {
			case <-ticker.C:
				select {
				case *rl.bucket <- struct{}{}:
				default:
				}
			}
		}
	}()
	return *rl.bucket
}

func (rl *RateLimit) GetToken(block bool) bool {
	if block {
		select {
		case <-*rl.bucket:
			return true
		}
	} else {
		select {
		case <-*rl.bucket:
			return true
		default:
			return false
		}
	}
}
