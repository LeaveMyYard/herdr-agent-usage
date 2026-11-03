/**
 * Tests for DecideBucket.
 */
package ratelimit

import "testing"

const resetsAt int64 = 1_800_000_000

func TestDecideBucket_Enters50(t *testing.T) {
	result := DecideBucket(WindowInput{UsedPercentage: 55, ResetsAt: resetsAt}, nil)
	if result.BucketToNotify == nil || *result.BucketToNotify != Bucket50 {
		t.Fatalf("bucketToNotify = %#v", result.BucketToNotify)
	}
	if result.NewState.NotifiedBucket == nil || *result.NewState.NotifiedBucket != Bucket50 {
		t.Fatalf("notifiedBucket = %#v", result.NewState.NotifiedBucket)
	}
}

func TestDecideBucket_NotifiesMostSevereOnly(t *testing.T) {
	result := DecideBucket(WindowInput{UsedPercentage: 92, ResetsAt: resetsAt}, nil)
	if result.BucketToNotify == nil || *result.BucketToNotify != Bucket10 {
		t.Fatalf("bucketToNotify = %#v, want 10", result.BucketToNotify)
	}
}

func TestDecideBucket_Boundary50(t *testing.T) {
	result := DecideBucket(WindowInput{UsedPercentage: 50, ResetsAt: resetsAt}, nil)
	if result.BucketToNotify == nil || *result.BucketToNotify != Bucket50 {
		t.Fatalf("bucketToNotify = %#v", result.BucketToNotify)
	}
}

func TestDecideBucket_EscalatesFrom50(t *testing.T) {
	b := Bucket50
	prev := &WindowState{ResetsAt: resetsAt, NotifiedBucket: &b, FailedNotifyAttempts: 0}
	result := DecideBucket(WindowInput{UsedPercentage: 92, ResetsAt: resetsAt}, prev)
	if result.BucketToNotify == nil || *result.BucketToNotify != Bucket10 {
		t.Fatalf("bucketToNotify = %#v, want 10", result.BucketToNotify)
	}
}

func TestDecideBucket_NoRenotifySameSeverity(t *testing.T) {
	b := Bucket20
	prev := &WindowState{ResetsAt: resetsAt, NotifiedBucket: &b, FailedNotifyAttempts: 0}
	result := DecideBucket(WindowInput{UsedPercentage: 82, ResetsAt: resetsAt}, prev)
	if result.BucketToNotify != nil {
		t.Fatalf("bucketToNotify = %#v, want nil", result.BucketToNotify)
	}
	if result.NewState.NotifiedBucket == nil || *result.NewState.NotifiedBucket != Bucket20 {
		t.Fatalf("notifiedBucket = %#v", result.NewState.NotifiedBucket)
	}
}

func TestDecideBucket_NewWindowResets(t *testing.T) {
	b := Bucket5
	prev := &WindowState{ResetsAt: resetsAt, NotifiedBucket: &b, FailedNotifyAttempts: 3}
	newResetsAt := resetsAt + 18_000
	result := DecideBucket(WindowInput{UsedPercentage: 55, ResetsAt: newResetsAt}, prev)
	if result.BucketToNotify == nil || *result.BucketToNotify != Bucket50 {
		t.Fatalf("bucketToNotify = %#v", result.BucketToNotify)
	}
	if result.NewState.ResetsAt != newResetsAt {
		t.Fatalf("resetsAt = %d", result.NewState.ResetsAt)
	}
	if result.NewState.FailedNotifyAttempts != 0 {
		t.Fatalf("failedNotifyAttempts = %d", result.NewState.FailedNotifyAttempts)
	}
}

func TestDecideBucket_ImprovedRemaining(t *testing.T) {
	b := Bucket20
	prev := &WindowState{ResetsAt: resetsAt, NotifiedBucket: &b, FailedNotifyAttempts: 0}
	result := DecideBucket(WindowInput{UsedPercentage: 60, ResetsAt: resetsAt}, prev)
	if result.BucketToNotify != nil {
		t.Fatalf("bucketToNotify = %#v, want nil", result.BucketToNotify)
	}
	if result.NewState.NotifiedBucket == nil || *result.NewState.NotifiedBucket != Bucket20 {
		t.Fatalf("notifiedBucket = %#v", result.NewState.NotifiedBucket)
	}
}

func TestDecideBucket_NoThreshold(t *testing.T) {
	result := DecideBucket(WindowInput{UsedPercentage: 30, ResetsAt: resetsAt}, nil)
	if result.BucketToNotify != nil {
		t.Fatalf("bucketToNotify = %#v, want nil", result.BucketToNotify)
	}
	if result.NewState.NotifiedBucket != nil {
		t.Fatalf("notifiedBucket = %#v, want nil", result.NewState.NotifiedBucket)
	}
}

func TestDecideBucketWithThresholds_UsesConfiguredBuckets(t *testing.T) {
	first := DecideBucketWithThresholds(
		WindowInput{UsedPercentage: 78, ResetsAt: resetsAt},
		nil,
		[]int{30, 10},
	)
	if first.BucketToNotify == nil || *first.BucketToNotify != Bucket("30") {
		t.Fatalf("bucketToNotify = %#v, want 30", first.BucketToNotify)
	}

	second := DecideBucketWithThresholds(
		WindowInput{UsedPercentage: 90, ResetsAt: resetsAt},
		&first.NewState,
		[]int{30, 10},
	)
	if second.BucketToNotify == nil || *second.BucketToNotify != Bucket10 {
		t.Fatalf("bucketToNotify = %#v, want 10", second.BucketToNotify)
	}
}

// Two sessions of one account report its reset seconds apart. Switching
// between them is the same window and must not re-fire the toast; a real
// reset (a whole window later) still starts over.
func TestDecideBucket_ResetJitterIsTheSameWindow(t *testing.T) {
	b50 := Bucket50
	previous := &WindowState{ResetsAt: 1794317308, NotifiedBucket: &b50}
	jitter := DecideBucket(WindowInput{UsedPercentage: 55, ResetsAt: 1794317313}, previous)
	if jitter.BucketToNotify != nil {
		t.Fatalf("re-fired %v on a 5 s reset jitter", *jitter.BucketToNotify)
	}
	next := DecideBucket(WindowInput{UsedPercentage: 55, ResetsAt: 1794317308 + 7*24*3600}, previous)
	if next.BucketToNotify == nil || *next.BucketToNotify != Bucket50 {
		t.Fatalf("a new window must notify again, got %+v", next)
	}
}
