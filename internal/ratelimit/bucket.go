/**
 * Decides rate-limit notification buckets from configured remaining-percent
 * thresholds while retaining default 50/20/10/5% behavior when absent.
 */
package ratelimit

import (
	"sort"
	"strconv"
)

func remainingPercentageOf(usedPercentage float64) float64 {
	return 100 - usedPercentage
}

func bucketsForThresholds(thresholds []int) []Bucket {
	if len(thresholds) == 0 {
		return BucketOrder
	}

	unique := make(map[int]struct{}, len(thresholds))
	values := make([]int, 0, len(thresholds))
	for _, threshold := range thresholds {
		if threshold < 1 || threshold > 100 {
			continue
		}
		if _, exists := unique[threshold]; exists {
			continue
		}
		unique[threshold] = struct{}{}
		values = append(values, threshold)
	}
	if len(values) == 0 {
		return BucketOrder
	}

	sort.Slice(values, func(i, j int) bool { return values[i] > values[j] })
	buckets := make([]Bucket, len(values))
	for i, threshold := range values {
		buckets[i] = Bucket(strconv.Itoa(threshold))
	}
	return buckets
}

// worstBucketFor returns the most severe configured bucket that remaining has entered.
func worstBucketFor(remainingPercentage float64, buckets []Bucket) *Bucket {
	var worst *Bucket
	for i := range buckets {
		bucket := buckets[i]
		threshold, err := strconv.Atoi(string(bucket))
		if err == nil && remainingPercentage <= float64(threshold) {
			worst = &bucket
		}
	}
	return worst
}

func severityRankOf(bucket *Bucket, buckets []Bucket) int {
	if bucket == nil {
		return -1
	}
	for i, candidate := range buckets {
		if candidate == *bucket {
			return i
		}
	}
	return -1
}

// sameWindowResetSlackSeconds is how far two readings' resetsAt may differ
// and still be the same window. Sessions of one account report a reset a few
// seconds apart (Codex rollouts written moments apart: 1794317308 vs
// 1794317313); an exact match would treat every switch between them as a new
// window and re-fire its toast. A real reset moves resetsAt by a whole window
// (hours or days), far beyond this.
const sameWindowResetSlackSeconds = 600

func isSameWindow(previous *WindowState, resetsAt int64) bool {
	if previous == nil {
		return false
	}
	d := previous.ResetsAt - resetsAt
	return d >= -sameWindowResetSlackSeconds && d <= sameWindowResetSlackSeconds
}

func decideBucket(input WindowInput, previous *WindowState, buckets []Bucket) BucketDecision {
	isNewWindow := !isSameWindow(previous, input.ResetsAt)
	var notifiedBucket *Bucket
	if !isNewWindow && previous != nil {
		notifiedBucket = previous.NotifiedBucket
	}

	remaining := remainingPercentageOf(input.UsedPercentage)
	currentWorst := worstBucketFor(remaining, buckets)
	shouldNotify := currentWorst != nil && severityRankOf(currentWorst, buckets) > severityRankOf(notifiedBucket, buckets)

	var newNotified *Bucket
	if shouldNotify {
		newNotified = currentWorst
	} else {
		newNotified = notifiedBucket
	}

	var bucketToNotify *Bucket
	if shouldNotify {
		bucketToNotify = currentWorst
	}

	return BucketDecision{
		NewState: WindowState{
			ResetsAt:             input.ResetsAt,
			NotifiedBucket:       newNotified,
			FailedNotifyAttempts: 0,
		},
		BucketToNotify: bucketToNotify,
	}
}

// DecideBucket uses the default 50/20/10/5% remaining thresholds.
func DecideBucket(input WindowInput, previous *WindowState) BucketDecision {
	return decideBucket(input, previous, BucketOrder)
}

// DecideBucketWithThresholds uses remaining-percent thresholds from plugin config.
func DecideBucketWithThresholds(input WindowInput, previous *WindowState, thresholds []int) BucketDecision {
	return decideBucket(input, previous, bucketsForThresholds(thresholds))
}
