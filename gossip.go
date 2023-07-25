package gossip_leaderelection

import (
	"context"
	"os"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

type gossip struct {
	delegate *delegate
	list     *memberlist.Memberlist
}

func NewGossip(config *memberlist.Config, addrs ...string) (KV, error) {
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
	if _, err = list.Join(addrs); err != nil {
		return nil, err
	}
	return &gossip{
		delegate: d,
		list:     list,
	}, nil
}

func (g *gossip) Get(ctx context.Context, key string) ([]byte, error) {
	logrus.WithField("key", key).Tracef("gossip.Get")
	b, ok := g.delegate.get(key)
	if !ok {
		return nil, os.ErrNotExist
	}
	return b, nil
}

func (g *gossip) Set(ctx context.Context, key string, value []byte) error {
	logrus.WithField("key", key).Tracef("gossip.Set")
	g.delegate.set(key, value)
	return nil
}

func (g *gossip) Delete(ctx context.Context, key string) error {
	logrus.WithField("key", key).Tracef("gossip.Delete")
	g.delegate.delete(key)
	return nil
}

func (g *gossip) Close() error {
	return g.list.Leave(time.Second)
}
