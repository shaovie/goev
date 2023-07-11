/*
// Use arrays for low range, and use maps for high range.
type evHandlerMap struct {
	arrSize int
	arr     []atomic.Pointer[EvHandler]
	sMap    sync.Map
}

func (m *evHandlerMap) Load(i int) EvHandler {
	if i < m.arrSize {
		e := m.arr[i].Load()
		return *e
	}
	if v, ok := m.sMap.Load(i); ok {
		return v.(EvHandler)
	}
	return nil
}
func (m *evHandlerMap) Store(i int, v EvHandler) {
	if i < m.arrSize {
		m.arr[i].Store(&v)
		return
	}
	m.sMap.Store(i, v)
}
func (m *evHandlerMap) Delete(i int) {
	if i < m.arrSize {
		m.arr[i].Store(nil)
		return
	}
	m.sMap.Delete(i)
}
                fd := int(ev.Fd)
				eh := ep.evHandlerMap.Load(fd)
				if eh == nil {
					continue // TODO add evOptions.debug? panic("evHandlerMap not found")
				}*/
