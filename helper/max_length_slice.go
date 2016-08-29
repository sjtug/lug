package helper

import "sync"

type MaxLengthStringSliceAdaptor struct {
	s      []string
	maxlen int
	lock   sync.Mutex
}

// Create an adaptor for given string slice
func NewMaxLengthStringSliceAdaptor(s []string, maxlen int) *MaxLengthStringSliceAdaptor {
	return &MaxLengthStringSliceAdaptor{
		s:      s,
		maxlen: maxlen,
	}
}

func (m MaxLengthStringSliceAdaptor) MaxLen() int {
	return m.maxlen
}

func (m *MaxLengthStringSliceAdaptor) Put(str string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.s = append(m.s, str)
	if len(m.s) > m.MaxLen() {
		m.s = m.s[1:]
	}
}

func (m MaxLengthStringSliceAdaptor) GetAll() []string {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]string, len(m.s))
	copy(result, m.s)
	return result
}

func (m MaxLengthStringSliceAdaptor) Len() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.s)
}

// Copy and init a new max length string slice
func NewMaxLengthSlice(s []string, maxlen int) *MaxLengthStringSliceAdaptor {
	news := make([]string, len(s))
	copy(news, s)
	return NewMaxLengthStringSliceAdaptor(news, maxlen)
}
