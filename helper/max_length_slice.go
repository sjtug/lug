package helper

import "sync"

// MaxLengthStringSliceAdaptor wrap the given slice with several methods for MaxLenSlice.
// Note that this adaptor does not own that slice at all, so keep the content of slice unchanged!
type MaxLengthStringSliceAdaptor struct {
	s      []string
	maxlen int
	lock   sync.Mutex
}

// NewMaxLengthStringSliceAdaptor creates an adaptor for given string slice
func NewMaxLengthStringSliceAdaptor(s []string, maxlen int) *MaxLengthStringSliceAdaptor {
	return &MaxLengthStringSliceAdaptor{
		s:      s,
		maxlen: maxlen,
	}
}

// MaxLen returns the maximum length of given maxlenslice
func (m MaxLengthStringSliceAdaptor) MaxLen() int {
	return m.maxlen
}

// Put adds a new item into slice, and may remove exceeded item(s)
func (m *MaxLengthStringSliceAdaptor) Put(str string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.s = append(m.s, str)
	if len(m.s) > m.MaxLen() {
		m.s = m.s[1:]
	}
}

// GetAll creates a duplicate of current slice and returns it
func (m MaxLengthStringSliceAdaptor) GetAll() []string {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]string, len(m.s))
	copy(result, m.s)
	return result
}

// Len returns current length of slice
func (m MaxLengthStringSliceAdaptor) Len() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.s)
}

// NewMaxLengthSlice copies and inits a new max length string slice
func NewMaxLengthSlice(s []string, maxlen int) *MaxLengthStringSliceAdaptor {
	news := make([]string, len(s))
	copy(news, s)
	return NewMaxLengthStringSliceAdaptor(news, maxlen)
}
