package clock

import (
	"sync/atomic"
	"time"
)

var now = time.Now().Truncate(time.Second).Unix()

func init() {
	go func() {
		for tick := time.Tick(time.Second); ; {
			atomic.StoreInt64(&now, (<-tick).Unix())
		}
	}()
}

func Unix() int64 {
	return atomic.LoadInt64(&now)
}
