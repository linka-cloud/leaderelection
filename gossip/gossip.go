package gossip

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"

	le "go.linka.cloud/leaderelection"
)

type GossipLock interface {
	io.Closer
	le.Lock
}

type gossipLock struct {
	*lock
	kv KV
}

func (l *gossipLock) Close() error {
	return l.kv.Close()
}

func New(ctx context.Context, config *memberlist.Config, lockName, id string, addrs ...string) (GossipLock, error) {
	kv, err := NewKV(ctx, config, addrs...)
	if err != nil {
		return nil, err
	}
	lock := &lock{kv: kv, name: lockName, id: id}
	return &gossipLock{kv: kv, lock: lock}, nil
}

type gossip struct {
	delegate *delegate
	list     *memberlist.Memberlist
}

func NewKV(_ context.Context, config *memberlist.Config, addrs ...string) (KV, error) {
	if config.Logger == nil {
		config.Logger = newLogger()
	}
	list, err := memberlist.Create(config)
	if err != nil {
		return nil, err
	}
	d := newDelegate(&memberlist.TransmitLimitedQueue{
		RetransmitMult: config.RetransmitMult,
		NumNodes:       list.NumMembers,
	})
	config.Delegate = d
	n, err := list.Join(addrs)
	if err != nil {
		return nil, err
	}
	if n > d.queue.RetransmitMult {
		d.queue.RetransmitMult = n
	}
	return &gossip{
		delegate: d,
		list:     list,
	}, nil
}

func (g *gossip) Get(ctx context.Context, key string) ([]byte, error) {
	logrus.WithField("key", key).Tracef("gossip.Get")
	b, ok, err := g.delegate.get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, os.ErrNotExist
	}
	return b, nil
}

func (g *gossip) Set(ctx context.Context, key string, value []byte) error {
	logrus.WithField("key", key).Tracef("gossip.Set")
	return g.delegate.set(ctx, key, value)
}

func (g *gossip) Delete(ctx context.Context, key string) error {
	logrus.WithField("key", key).Tracef("gossip.Delete")
	return g.delegate.delete(ctx, key)
}

func (g *gossip) Close() error {
	return g.list.Leave(time.Second)
}