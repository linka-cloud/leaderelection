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
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

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
