// Copyright 2023 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gossip

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"go.linka.cloud/grpc-toolkit/logger"
)

var _ memberlist.Delegate = (*delegate)(nil)

type delegate struct {
	queue *memberlist.TransmitLimitedQueue
	kmu   sync.RWMutex
	kv    map[string]*kv
	log   logger.Logger
}

func newDelegate(ctx context.Context, queue *memberlist.TransmitLimitedQueue) *delegate {
	return &delegate{
		queue: queue,
		kv:    make(map[string]*kv),
		log:   logger.C(ctx),
	}
}

func (d *delegate) NodeMeta(_ int) []byte {
	return nil
}

func (d *delegate) NotifyMsg(b []byte) {
	log := d.log.WithField("method", "delegate.NotifyMsg")

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
	d.queue.QueueBroadcast(&broadcast{payload: b, action: a})
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	log := d.log.WithFields("method", "delegate.GetBroadcasts", "overhead", overhead, "limit", limit)
	out := d.queue.GetBroadcasts(overhead, limit)
	log.Tracef("length: %d", len(out))
	return out
}

func (d *delegate) LocalState(join bool) []byte {
	log := d.log.WithFields("method", "delegate.LocalState", "join", join)
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
	log := d.log.WithFields("method", "delegate.MergeRemoteState", "join", join)
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
	log := logger.C(ctx).WithFields("method", "delegate.get", "key", key)
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
	log := logger.C(ctx).WithFields("method", "delegate.get", "key", key)
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
	a := &action{typ: actionTypeSet, key: key, value: value, time: k.time}
	b := a.Encode()
	d.queue.QueueBroadcast(&broadcast{payload: b, action: a})
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
	a := &action{typ: actionTypeDelete, key: key, time: time.Now().Truncate(time.Millisecond)}
	b := a.Encode()
	d.queue.QueueBroadcast(&broadcast{payload: b, action: a})
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
