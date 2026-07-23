package routinemanager

type Manager struct {
	channel     chan uint16
	maxRoutines uint16
}

func NewManager(maxRoutines uint16) *Manager {
	m := &Manager{
		maxRoutines: maxRoutines,
		channel:     make(chan uint16, maxRoutines),
	}
	for i := uint16(0); i < maxRoutines; i++ {
		m.channel <- i
	}
	return m
}

func (m *Manager) Lock() uint16 {
	return <-m.channel
}

func (m *Manager) Unlock(i uint16) {
	m.channel <- i
}
