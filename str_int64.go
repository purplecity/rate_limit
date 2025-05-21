package rate_limit

import (
	"sync"
)

type StringInt64Map struct {
	Lock *sync.RWMutex
	BM   map[string]int64
}

func NewStringInt64Map() *StringInt64Map {
	return &StringInt64Map{
		Lock: new(sync.RWMutex),
		BM:   make(map[string]int64),
	}
}

func (m *StringInt64Map) Get(k string) int64 {
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	if val, ok := m.BM[k]; ok {
		return val
	}
	return 0
}

func (m *StringInt64Map) Set(k string, v int64) bool {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	if val, ok := m.BM[k]; !ok {
		m.BM[k] = v
	} else if val != v {
		m.BM[k] = v
	} else {
		return false
	}
	return true
}

func (m *StringInt64Map) Check(k string) bool {
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	_, ok := m.BM[k]
	return ok
}

func (m *StringInt64Map) Delete(k string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	delete(m.BM, k)
}

func (m *StringInt64Map) Items() map[string]int64 {
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	r := make(map[string]int64)
	for k, v := range m.BM {
		r[k] = v
	}
	return r
}

func (m *StringInt64Map) SomeOfItems(n int) map[string]int64 {
	length := m.Count()
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	r := make(map[string]int64)
	count := 0
	for k, v := range m.BM {
		if count > length/n {
			break
		}
		r[k] = v
		count++
	}
	return r
}

func (m *StringInt64Map) SomeItems(n int) map[string]int64 {
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	r := make(map[string]int64)
	count := 0
	for k, v := range m.BM {
		if count > n {
			break
		}
		r[k] = v
		count++
	}
	return r
}

func (m *StringInt64Map) Keys() []string {
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	r := []string{}
	for k := range m.BM {
		r = append(r, k)
	}
	return r
}

func (m *StringInt64Map) Count() int {
	m.Lock.RLock()
	defer m.Lock.RUnlock()
	return len(m.BM)
}

func (m *StringInt64Map) AddOne(k string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	before, _ := m.BM[k]
	m.BM[k] = before + 1
}

func (m *StringInt64Map) SubOne(k string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	before, _ := m.BM[k]
	after := before - 1
	if after <= 0 {
		m.BM[k] = 0
	} else {
		m.BM[k] = after
	}
}
