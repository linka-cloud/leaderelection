package gossip_leaderelection

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type RWMutex struct {
	mu sync.RWMutex
}

func (m *RWMutex) Lock() {
	logrus.Tracef("Locking mutex")
	s := time.Now()
	m.mu.Lock()
	logrus.Tracef("Locked mutex in %s", time.Since(s))
}

func (m *RWMutex) Unlock() {
	logrus.Tracef("Unlocking mutex")
	m.mu.Unlock()
}

func (m *RWMutex) RLock() {
	logrus.Tracef("RLocking mutex")
	s := time.Now()
	m.mu.RLock()
	logrus.Tracef("RLocked mutex in %s", time.Since(s))
}

func (m *RWMutex) RUnlock() {
	logrus.Tracef("RUnlocking mutex")
	m.mu.RUnlock()
}
