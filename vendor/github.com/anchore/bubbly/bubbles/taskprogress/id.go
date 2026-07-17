package taskprogress

import "sync"

// Internal ID management for text inputs. Necessary for blink integrity when
// multiple text inputs are involved.
var (
	lastID int
	idMtx  sync.Mutex
)

// Return the next ID we should use on the Model.
func nextID() int {
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}
