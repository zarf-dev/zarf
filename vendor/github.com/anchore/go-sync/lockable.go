package sync

import (
	"reflect"
	"sync"
)

type UnlockFunc func()

// Lockable implementors provide the ability to RW lock a resource
type Lockable interface {
	Lock() (unlock UnlockFunc)
	RLock() (unlock UnlockFunc)
}

// Locking is a utility to add Lockable behavior to a struct
type Locking struct {
	lock sync.RWMutex
}

var _ Lockable = (*Locking)(nil)

func (l *Locking) Lock() (unlock UnlockFunc) {
	l.lock.Lock()
	return l.lock.Unlock
}

func (l *Locking) RLock() (unlock UnlockFunc) {
	l.lock.RLock()
	return l.lock.RUnlock
}

func (l *Locking) IsExclusiveLock(unlockFunc UnlockFunc) (exclusive bool) {
	unlock := reflect.ValueOf(l.lock.Unlock).Pointer()
	f := reflect.ValueOf(unlockFunc).Pointer()
	return unlock == f
}
