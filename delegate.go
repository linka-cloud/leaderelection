package gossip_leaderelection

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

var _ memberlist.Delegate = (*delegate)(nil)

type delegate struct {
	queue *memberlist.TransmitLimitedQueue
	kmu   RWMutex
	kv    map[string][]byte

	seen   map[string]time.Time
	smu    sync.RWMutex
	maxAge time.Duration
}

func newDelegate(queue *memberlist.TransmitLimitedQueue) *delegate {
	return &delegate{
		queue:  queue,
		kv:     make(map[string][]byte),
		seen:   make(map[string]time.Time),
		maxAge: 10 * time.Second,
	}
}

func (d *delegate) NodeMeta(limit int) []byte {
	return nil
}

func (d *delegate) NotifyMsg(bytes []byte) {
	logrus.Tracef("delegate.NotifyMsg")
	d.smu.RLock()
	h := hash(bytes)
	_, ok := d.seen[h]
	d.smu.RUnlock()
	if ok {
		return
	}
	logrus.Debugf("delegate.NotifyMsg: new message: %s", h)
	d.smu.Lock()
	d.seen[h] = time.Now()
	d.smu.Unlock()
	a := &action{}
	if err := a.Decode(bytes); err != nil {
		panic(err)
	}
	d.kmu.Lock()
	defer d.kmu.Unlock()
	switch a.typ {
	case actionTypeSet:
		d.kv[a.key] = a.value
	case actionTypeDelete:
		delete(d.kv, a.key)
	}
	// queue broadcast to be sent to other nodes
	d.queue.QueueBroadcast(&broadcast{payload: bytes})
	d.clean()
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	// logrus.WithField("overhead", overhead).WithField("limit", limit).Tracef("delegate.GetBroadcasts")
	return d.queue.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	logrus.Tracef("delegate.LocalState join=%t", join)
	d.kmu.RLock()
	defer d.kmu.RUnlock()
	var b []byte
	for k, v := range d.kv {
		b = append(b, (&kv{key: k, value: v}).Encode()...)
	}
	return b
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	logrus.WithField("join", join).Tracef("delegate.MergeRemoteState")
	// if !join {
	// 	return
	// }
	d.kmu.Lock()
	defer d.kmu.Unlock()
	for len(buf) > 0 {
		k := &kv{}
		n, err := k.Decode(buf)
		if err != nil {
			panic(err)
		}
		logrus.WithField("key", k.key).WithField("value", string(k.value)).Tracef("delegate.MergeRemoteState")
		d.kv[k.key] = k.value
		buf = buf[n:]
	}
}

func (d *delegate) get(key string) ([]byte, bool) {
	d.kmu.RLock()
	defer d.kmu.RUnlock()
	b, ok := d.kv[key]
	return b[:], ok
}

func (d *delegate) set(key string, value []byte) {
	d.kmu.Lock()
	if bytes.Equal(d.kv[key], value) {
		d.kmu.Unlock()
		return
	}
	logrus.WithField("key", key).Debugf("delegate.set: %s", string(value))
	d.kv[key] = value
	d.kmu.Unlock()
	b := (&action{typ: actionTypeSet, key: key, value: value}).Encode()
	h := hash(b)
	d.smu.Lock()
	d.seen[h] = time.Now()
	d.smu.Unlock()
	d.queue.QueueBroadcast(&broadcast{payload: b})
}

func (d *delegate) delete(key string) {
	d.kmu.Lock()
	if _, ok := d.kv[key]; !ok {
		d.kmu.Unlock()
		return
	}
	delete(d.kv, key)
	d.kmu.Unlock()
	b := (&action{typ: actionTypeDelete, key: key}).Encode()
	d.smu.Lock()
	h := hash(b)
	d.smu.Unlock()
	d.seen[h] = time.Now()
	d.queue.QueueBroadcast(&broadcast{payload: b})
}

func (d *delegate) clean() {
	for k, v := range d.seen {
		if time.Since(v) > d.maxAge {
			delete(d.seen, k)
		}
	}
}

func hash(msg []byte) string {
	h := sha256.New()
	h.Write(msg)
	return hex.EncodeToString(h.Sum(nil))
}
