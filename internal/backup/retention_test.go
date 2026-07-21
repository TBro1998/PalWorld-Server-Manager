package backup

import (
	"sort"
	"testing"
	"time"
)

func mkItems(base time.Time, n int) []RetentionItem {
	items := make([]RetentionItem, n)
	for i := 0; i < n; i++ {
		// item i created i days before base (item 0 newest).
		items[i] = RetentionItem{ID: int64(i + 1), CreatedAt: base.AddDate(0, 0, -i)}
	}
	return items
}

func sortedIDs(ids []int64) []int64 {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func TestPruneNoLimits(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	if got := Prune(mkItems(now, 5), 0, 0, now); got != nil {
		t.Fatalf("expected no pruning with 0/0, got %v", got)
	}
}

func TestPruneByCount(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	// 5 items, keep newest 2 → prune the 3 oldest (ids 3,4,5).
	got := sortedIDs(Prune(mkItems(now, 5), 2, 0, now))
	want := []int64{3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("count prune: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("count prune: got %v want %v", got, want)
		}
	}
}

func TestPruneByDays(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	// items 0..5 at now-0d..now-5d; keepDays=3 prunes anything strictly older
	// than now-3d, i.e. now-4d and now-5d (ids 5 and 6).
	items := mkItems(now, 6)
	got := sortedIDs(Prune(items, 0, 3, now))
	want := []int64{5, 6}
	if len(got) != len(want) {
		t.Fatalf("days prune: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("days prune: got %v want %v", got, want)
		}
	}
}

func TestPruneStricterOfBoth(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	// 6 items. keepCount=4 would prune ids 5,6. keepDays=2 would prune ids 4,5,6
	// (older than now-2d). Union (stricter) = 4,5,6.
	items := mkItems(now, 6)
	got := sortedIDs(Prune(items, 4, 2, now))
	want := []int64{4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("combined prune: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("combined prune: got %v want %v", got, want)
		}
	}
}
