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
	"context"
	"io"
	"os"
	"time"

	"github.com/hashicorp/memberlist"
	"go.linka.cloud/grpc-toolkit/logger"
	"go.uber.org/multierr"

	le "go.linka.cloud/leaderelection"
	"go.linka.cloud/leaderelection/gossip/internal/dns"
)

type Lock interface {
	io.Closer
	le.Lock

	Memberlist() *memberlist.Memberlist
	UpdateMeta(ctx context.Context, meta []byte) error
}

type gossipLock struct {
	*lock
	kv *kvstore
}

func (l *gossipLock) UpdateMeta(ctx context.Context, meta []byte) error {
	l.kv.delegate.setMeta(meta)
	var t time.Duration
	if d, ok := ctx.Deadline(); ok {
		t = time.Until(d)
	}
	return l.kv.list.UpdateNode(t)
}

func (l *gossipLock) Memberlist() *memberlist.Memberlist {
	return l.kv.list
}

func (l *gossipLock) Close() error {
	return l.kv.Close()
}

func New(ctx context.Context, config *memberlist.Config, lockName, id string, meta []byte, addrs ...string) (Lock, error) {
	kv, err := newKVStore(ctx, config, meta, addrs...)
	if err != nil {
		return nil, err
	}
	lock := &lock{kv: kv, name: lockName, id: id}
	return &gossipLock{kv: kv, lock: lock}, nil
}

type kvstore struct {
	delegate *delegate
	list     *memberlist.Memberlist
}

func NewKVStore(ctx context.Context, config *memberlist.Config, meta []byte, addrs ...string) (KV, error) {
	return newKVStore(ctx, config, meta, addrs...)
}

func newKVStore(ctx context.Context, config *memberlist.Config, meta []byte, addrs ...string) (*kvstore, error) {
	if config.Logger == nil {
		config.Logger = newLogger(ctx)
	}
	p := dns.NewProvider(ctx, dns.MiekgdnsResolverType)
	if err := p.Resolve(ctx, addrs); err != nil {
		return nil, err
	}
	addrs = p.Addresses()
	if config.RetransmitMult < len(addrs) {
		config.RetransmitMult = len(addrs)
	}
	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}
	list.LocalNode().Meta = meta
	d := newDelegate(ctx, &memberlist.TransmitLimitedQueue{
		RetransmitMult: config.RetransmitMult,
		NumNodes:       list.NumMembers,
	})
	config.Delegate = d
	n, err := list.Join(addrs)
	if err != nil {
		return nil, err
	}
	if d.queue.RetransmitMult < n {
		d.queue.RetransmitMult = n
	}
	return &kvstore{
		delegate: d,
		list:     list,
	}, nil
}

func (g *kvstore) Get(ctx context.Context, key string) ([]byte, error) {
	logger.C(ctx).WithField("key", key).Tracef("gossip.Get")
	b, ok, err := g.delegate.get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, os.ErrNotExist
	}
	return b, nil
}

func (g *kvstore) Set(ctx context.Context, key string, value []byte) error {
	logger.C(ctx).WithField("key", key).Tracef("gossip.Set")
	return g.delegate.set(ctx, key, value)
}

func (g *kvstore) Delete(ctx context.Context, key string) error {
	logger.C(ctx).WithField("key", key).Tracef("gossip.Delete")
	return g.delegate.delete(ctx, key)
}

func (g *kvstore) Close() error {
	return multierr.Combine(g.list.Leave(time.Second), g.list.Shutdown())
}
