package redact

import (
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/scylladb/go-set/strset"
)

type Store interface {
	Redactor
	StoreWriter
}

type Redactor interface {
	RedactString(string) string
	identifiable
}

type StoreWriter interface {
	Add(value ...string)
	identifiable
}

type identifiable interface {
	id() string
}

// redactorCollection holds a list of redactors, applying all of them to Redact* calls
type redactorCollection []Redactor

var _ Redactor = (*redactorCollection)(nil)

func newRedactorCollection(readers ...Redactor) Redactor {
	collection := make(redactorCollection, 0, len(readers))
	ids := strset.New()
	addReader := func(rs ...Redactor) {
		for _, r := range rs {
			if ids.Has(r.id()) {
				continue
			}
			collection = append(collection, r)
			ids.Add(r.id())
		}
	}
	for _, r := range readers {
		if rs, ok := r.(redactorCollection); ok {
			addReader(rs...)
		} else {
			addReader(r)
		}
	}
	return collection
}

func (c redactorCollection) RedactString(s string) string {
	for _, r := range c {
		s = r.RedactString(s)
	}
	return s
}

func (c redactorCollection) id() (val string) {
	for _, r := range c {
		val += r.id()
	}
	return val
}

// store maintains a list of redactions, and implements Redactor Redact* methods
type store struct {
	redactions *strset.Set
	lock       *sync.RWMutex
	_id        string
}

var _ Store = (*store)(nil)

func NewStore(values ...string) Store {
	return &store{
		redactions: strset.New(values...),
		lock:       &sync.RWMutex{},
		_id:        uuid.New().String(),
	}
}

func (w *store) id() string {
	return w._id
}

func (w *store) Add(values ...string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	for _, value := range values {
		if len(value) <= 1 {
			// smallest possible redaction string must be larger than 1 character
			continue
		}
		w.redactions.Add(value)
	}
}

func (w *store) values() []string {
	w.lock.RLock()
	defer w.lock.RUnlock()
	return w.redactions.List()
}

func (w *store) RedactString(str string) string {
	for _, s := range w.values() {
		// note: we don't use the length of the redaction string to determine the replacement string, as even the length could be considered sensitive
		str = strings.ReplaceAll(str, s, strings.Repeat("*", 7))
	}
	return str
}
