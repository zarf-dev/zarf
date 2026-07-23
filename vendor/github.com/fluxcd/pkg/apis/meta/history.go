/*
Copyright 2025 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package meta

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HistoryMaxSize defines the maximum number of snapshots to keep in history.
	HistoryMaxSize = 5
)

// History represents a collection of snapshots that tracks the reconciliation
// history of a group of resources, automatically sorted by last reconciled timestamp.
type History []Snapshot

// Len returns the length of the history slice.
func (h History) Len() int { return len(h) }

// Less reports whether the element with index i should sort before the element with index j.
// Sorts by LastReconciled in descending order (most recent first).
func (h History) Less(i, j int) bool {
	return h[i].LastReconciled.After(h[j].LastReconciled.Time)
}

// Swap swaps the elements with indexes i and j.
func (h History) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Latest returns the most recent snapshot from the history.
// Returns nil if the history is empty.
func (h History) Latest() *Snapshot {
	if len(h) == 0 {
		return nil
	}
	return &h[0]
}

// truncate keeps only the latest snapshots in the history up to HistoryMaxSize.
// Since the history is maintained with most recent first, we simply truncate from the end.
func (h *History) truncate() {
	if len(*h) <= HistoryMaxSize {
		return
	}
	*h = (*h)[:HistoryMaxSize]
}

// Upsert adds a new snapshot to the history or updates an existing one
// with the same digest and status. The most recent snapshot is moved to the front of the array.
// When adding new snapshots, the history is automatically truncated to HistoryMaxSize.
func (h *History) Upsert(digest string, timestamp time.Time, duration time.Duration, status string, metadata map[string]string) {
	defer h.truncate()

	now := metav1.NewTime(timestamp)
	durationMeta := metav1.Duration{Duration: duration}

	// Look for existing snapshot with same digest and status
	for i := range *h {
		if (*h)[i].Digest == digest && (*h)[i].LastReconciledStatus == status {
			// Update existing snapshot
			(*h)[i].LastReconciled = now
			(*h)[i].LastReconciledDuration = durationMeta
			(*h)[i].TotalReconciliations++
			(*h)[i].Metadata = metadata
			// Move to front if not already there
			if i > 0 {
				snapshot := (*h)[i]
				copy((*h)[1:i+1], (*h)[0:i])
				(*h)[0] = snapshot
			}
			return
		}
	}

	// Add new snapshot at the front
	newSnapshot := Snapshot{
		Digest:                 digest,
		FirstReconciled:        now,
		LastReconciled:         now,
		LastReconciledDuration: durationMeta,
		LastReconciledStatus:   status,
		TotalReconciliations:   1,
		Metadata:               metadata,
	}

	*h = append([]Snapshot{newSnapshot}, *h...)
}

// Snapshot represents a point-in-time record of a group of resources reconciliation,
// including timing information, status, and a unique digest identifier.
type Snapshot struct {
	// Digest is the checksum in the format `<algo>:<hex>` of the resources in this snapshot.
	// +required
	Digest string `json:"digest"`

	// FirstReconciled is the time when this revision was first reconciled to the cluster.
	// +required
	FirstReconciled metav1.Time `json:"firstReconciled"`

	// LastReconciled is the time when this revision was last reconciled to the cluster.
	// +required
	LastReconciled metav1.Time `json:"lastReconciled"`

	// LastReconciledDuration is time it took to reconcile the resources in this revision.
	// +kubebuilder:validation:Type=string
	// +required
	LastReconciledDuration metav1.Duration `json:"lastReconciledDuration"`

	// LastReconciledStatus is the status of the last reconciliation.
	// +required
	LastReconciledStatus string `json:"lastReconciledStatus"`

	// TotalReconciliations is the total number of reconciliations that have occurred for this snapshot.
	// + required
	TotalReconciliations int64 `json:"totalReconciliations"`

	// Metadata contains additional information about the snapshot.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}
