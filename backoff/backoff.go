package backoff

import (
	"errors"
	"math"
	"time"
)

// ErrMaxBackoffReached is returned when the maximum backoff count has been reached.
var ErrMaxBackoffReached = errors.New("backoff: max backoff reached")

// NewBackoff creates a backoff function.
// Retry Backoff Total elapsed
// 0     0       0
// 1     200     0.2
// 2     400     0.6
// 3     800     1.4
// 4     1600    3
// 5     3200    6.2
// 6     6400    12.6
// 7     12800   25.4
//
// Usage:
//
//	 bo := backoff.New(10)
//		for {
//		  done, err := checkStatus()
//		  if err != nil {
//		     return err
//		  }
//		  if !done {
//		     break
//		  }
//		  if err := bo(); err != nil {
//		     return err
//		  }
//		}
func New(maxRetries int) func() error {
	var retry int
	return func() error {
		if retry > maxRetries {
			return ErrMaxBackoffReached
		}
		if retry == 0 {
			return nil
		}
		retry++
		t := math.Pow(2.0, float64(retry))
		sleepFor := time.Duration(t) * (100 * time.Millisecond)
		time.Sleep(sleepFor)
		return nil
	}
}
