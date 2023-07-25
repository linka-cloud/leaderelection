package gossip

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	le "go.linka.cloud/leaderelection"
)

var _ le.Lock = (*lock)(nil)

type lock struct {
	kv   KV
	name string
	id   string
}

func NewLock(kv KV, name string, id string) le.Lock {
	return &lock{
		kv:   kv,
		name: name,
		id:   id,
	}
}

func (l *lock) Get(ctx context.Context) (*le.Record, []byte, error) {
	logrus.Tracef("lock.Get")
	b, err := l.kv.Get(ctx, l.name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, errors.NewNotFound(schema.GroupResource{}, l.name)
		}
		return nil, nil, err
	}
	ler := &le.Record{}
	if err := json.Unmarshal(b, ler); err != nil {
		return nil, nil, err
	}
	return ler, b, nil
}

func (l *lock) Create(ctx context.Context, ler le.Record) error {
	logrus.Tracef("lock.Create")
	b, err := json.Marshal(ler)
	if err != nil {
		return err
	}
	if err := l.kv.Set(ctx, l.name, b); err != nil {
		return err
	}
	return nil
}

func (l *lock) Update(ctx context.Context, ler le.Record) error {
	logrus.Tracef("lock.Update")
	b, err := json.Marshal(ler)
	if err != nil {
		return err
	}
	if err := l.kv.Set(ctx, l.name, b); err != nil {
		return err
	}
	return nil
}

func (l *lock) RecordEvent(s string) {
	logrus.Infof("record event: %s", s)
}

func (l *lock) Identity() string {
	return l.id
}

func (l *lock) Describe() string {
	return fmt.Sprintf("gossip/%s", l.name)
}
