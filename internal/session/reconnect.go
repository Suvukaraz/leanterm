package session

import (
	"time"
)

// BackoffDuration returns the exponential backoff duration according to Section 8.7:
// attempt 1 -> 1s, attempt 2 -> 2s, attempt 3 -> 5s, attempt 4+ -> 10s.
func BackoffDuration(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 1 * time.Second
	case 2:
		return 2 * time.Second
	case 3:
		return 5 * time.Second
	default:
		return 10 * time.Second
	}
}
