package gossip

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

var _ memberlist.Delegate = (*delegate)(nil)

type delegate struct {
	queue *memberlist.TransmitLimitedQueue
	kmu   sync.RWMutex
	kv    map[string]*kv
}

func newDelegate(queue *memberlist.TransmitLimitedQueue) *delegate {
	return &delegate{
		queue: queue,
		kv:    make(map[string]*kv),
	}
}

func (d *delegate) NodeMeta(_ int) []byte {
	return nil
}

func (d *delegate) NotifyMsg(b []byte) {
	log := logrus.WithField("method", "delegate.NotifyMsg")

	a := &action{}
	if err := a.Decode(b); err != nil {
		panic(err)
	}
	log = log.WithField("key", a.key).WithField("type", a.typ)
	d.kmu.Lock()
	defer d.kmu.Unlock()
	if v, ok := d.kv[a.key]; ok {
		if a.time.Before(v.time) {
			log.Tracef("skipping old message: %v | %v", a.time, v.time)
			return
		}
		if bytes.Equal(v.value, a.value) {
			if maybeClose(v.confirmed) {
				log.Debugf("confirmed")
			}
			return
		}
		log.Debugf("overriding value")
	}
	log.Debugf("new message: %s", string(a.value))
	switch a.typ {
	case actionTypeSet:
		k := &kv{key: a.key, value: a.value, confirmed: make(chan struct{}), time: a.time}
		d.kv[a.key] = k
		close(k.confirmed)
	case actionTypeDelete:
		delete(d.kv, a.key)
	}
	// queue broadcast to be sent to other nodes
	d.queue.QueueBroadcast(&broadcast{payload: b})
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	log := logrus.
		WithField("method", "delegate.GetBroadcasts").
		WithField("overhead", overhead).
		WithField("limit", limit)
	out := d.queue.GetBroadcasts(overhead, limit)
	log.Tracef("length: %d", len(out))
	return out
}

func (d *delegate) LocalState(join bool) []byte {
	log := logrus.WithField("method", "delegate.LocalState").WithFields(logrus.Fields{"join": join})
	log.Trace()
	d.kmu.RLock()
	defer d.kmu.RUnlock()
	var b []byte
	for _, v := range d.kv {
		b = append(b, v.Encode()...)
	}
	return b
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	log := logrus.WithField("method", "delegate.MergeRemoteState").WithFields(logrus.Fields{"join": join})
	d.kmu.Lock()
	defer d.kmu.Unlock()
	for len(buf) > 0 {
		k := &kv{
			confirmed: make(chan struct{}),
		}
		n, err := k.Decode(buf)
		if err != nil {
			panic(err)
		}
		buf = buf[n:]
		log := log.WithField("key", k.key).WithField("value", string(k.value))
		log.Trace()
		if v, ok := d.kv[k.key]; ok {
			if k.time.Before(v.time) {
				log.Tracef("skipping old message")
				continue
			}
			if bytes.Equal(v.value, k.value) {
				if maybeClose(v.confirmed) {
					log.Debugf("confirmed: %s: %s", k.key, string(k.value))
				}
				continue
			}
		}
		d.kv[k.key] = k
		maybeClose(k.confirmed)
	}
}

func (d *delegate) get(ctx context.Context, key string) ([]byte, bool, error) {
	log := logrus.WithField("method", "delegate.get").WithFields(logrus.Fields{"key": key})
	log.Trace()
	d.kmu.RLock()
	b, ok := d.kv[key]
	d.kmu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	select {
	case <-b.confirmed:
		return b.value, ok, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

func (d *delegate) set(ctx context.Context, key string, value []byte) error {
	log := logrus.WithField("method", "delegate.get").WithFields(logrus.Fields{"key": key})
	d.kmu.Lock()
	if kv, ok := d.kv[key]; ok && bytes.Equal(kv.value, value) {
		defer d.kmu.Unlock()
		select {
		case <-kv.confirmed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	log.Debugf("value: %s", string(value))
	k := &kv{key: key, value: value, confirmed: make(chan struct{}), time: time.Now().Truncate(time.Millisecond)}
	d.kv[key] = k
	d.kmu.Unlock()
	b := (&action{typ: actionTypeSet, key: key, value: value, time: k.time}).Encode()
	d.queue.QueueBroadcast(&broadcast{payload: b})
	if d.queue.NumNodes() == 1 {
		maybeClose(k.confirmed)
		log.Debugf("single node: confirmed")
	}
	tk := time.NewTicker(5 * time.Millisecond)
	for {
		select {
		case <-tk.C:
			if d.queue.NumNodes() == 1 {
				maybeClose(k.confirmed)
				log.Debugf("single node: confirmed")
			}
		case <-k.confirmed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *delegate) delete(_ context.Context, key string) error {
	d.kmu.Lock()
	if _, ok := d.kv[key]; !ok {
		d.kmu.Unlock()
		return nil
	}
	delete(d.kv, key)
	d.kmu.Unlock()
	b := (&action{typ: actionTypeDelete, key: key, time: time.Now().Truncate(time.Millisecond)}).Encode()
	d.queue.QueueBroadcast(&broadcast{payload: b})
	return nil
}

func maybeClose[T any](c chan T) bool {
	select {
	case <-c:
		return false
	default:
		close(c)
		return true
	}
}