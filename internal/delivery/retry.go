package delivery

import "time"

var DefaultRetrySchedule = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	8 * time.Hour,
	24 * time.Hour,
}

func NextRetryTime(attemptCount int, schedule []time.Duration) *time.Time {
	// attemptCount is 1-indexed (attempt 1 just happened, so index 0 gives delay before attempt 2)
	idx := attemptCount - 1
	if idx < 0 || idx >= len(schedule) {
		return nil // no more retries
	}
	t := time.Now().UTC().Add(schedule[idx])
	return &t
}

func IsSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
