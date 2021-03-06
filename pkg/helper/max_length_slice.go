package helper

import (
	"sync"
)

// MaxLengthStringSliceAdaptor wrap the given slice with several methods for MaxLenSlice.
// Note that this adaptor does not own that slice at all, so keep the content of slice unchanged!
type MaxLengthStringSliceAdaptor struct {
	s      []string
	maxlen int
	lock   sync.RWMutex
}

// NewMaxLengthStringSliceAdaptor creates an adaptor for given string slice
func NewMaxLengthStringSliceAdaptor(s []string, maxlen int) *MaxLengthStringSliceAdaptor {
	result := &MaxLengthStringSliceAdaptor{
		s:      s,
		maxlen: maxlen,
	}
	result.removeExceededItems()
	return result
}

// MaxLen returns the maximum length of given maxlenslice
func (m *MaxLengthStringSliceAdaptor) MaxLen() int {
	return m.maxlen
}

// Use it when you acquire the lock
func (m *MaxLengthStringSliceAdaptor) removeExceededItems() {
	if len(m.s) > m.MaxLen() {
		m.s = m.s[len(m.s)-m.MaxLen():]
	}
}

// Put adds a new item into slice, and may remove exceeded item(s)
func (m *MaxLengthStringSliceAdaptor) Put(str string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.s = append(m.s, str)
	m.removeExceededItems()
}

// GetAll creates a duplicate of current slice and returns it
func (m *MaxLengthStringSliceAdaptor) GetAll() []string {
	m.lock.RLock()
	defer m.lock.RUnlock()
	result := make([]string, len(m.s))
	copy(result, m.s)
	return result
}

// Len returns current length of slice
func (m *MaxLengthStringSliceAdaptor) Len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.s)
}

// NewMaxLengthSlice copies and inits a new max length string slice
func NewMaxLengthSlice(s []string, maxlen int) *MaxLengthStringSliceAdaptor {
	news := make([]string, len(s))
	copy(news, s)
	return NewMaxLengthStringSliceAdaptor(news, maxlen)
}
