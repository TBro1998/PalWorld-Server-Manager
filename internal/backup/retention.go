package backup

import (
	"sort"
	"time"
)

// RetentionItem is the minimal view of a backup the retention policy needs: its
// identity and creation time. Callers adapt their DB rows to this.
type RetentionItem struct {
	ID        int64
	CreatedAt time.Time
}

// Prune computes which backups to delete given a retention policy, returning
// their IDs. Two independent dimensions apply, and a backup is pruned if it
// violates EITHER (the stricter combined result):
//
//   - keepCount > 0: keep only the newest keepCount backups; older ones prune.
//   - keepDays  > 0: prune anything older than keepDays days from now.
//
// A dimension set to 0 means "no limit" on that axis. When both are 0 nothing
// is pruned. now is passed in for testability.
func Prune(items []RetentionItem, keepCount, keepDays int, now time.Time) []int64 {
	if keepCount <= 0 && keepDays <= 0 {
		return nil
	}

	// Sort a copy newest-first so index-based count pruning is well defined.
	sorted := make([]RetentionItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	var cutoff time.Time
	if keepDays > 0 {
		cutoff = now.AddDate(0, 0, -keepDays)
	}

	var toDelete []int64
	for i, it := range sorted {
		prune := false
		if keepCount > 0 && i >= keepCount {
			prune = true
		}
		if keepDays > 0 && it.CreatedAt.Before(cutoff) {
			prune = true
		}
		if prune {
			toDelete = append(toDelete, it.ID)
		}
	}
	return toDelete
}
