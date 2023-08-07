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

package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/sirupsen/logrus"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	le "go.linka.cloud/leaderelection"
)

var _ le.Lock = (*lock)(nil)

type lock struct {
	name string
	auth transport.AuthMethod
	repo *git.Repository
	id   string
	mu   sync.RWMutex
}

func New(ctx context.Context, name, url string, auth transport.AuthMethod, id string) (le.Lock, error) {
	r, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:  url,
		Auth: auth,
	})
	if errorContains(err, "remote repository is empty") {
		r, err = git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			return nil, fmt.Errorf("failed to init repo: %w", err)
		}
		_, err = r.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{url},
		})
	}
	if err != nil {
		return nil, err
	}
	return &lock{name: name, auth: auth, repo: r, id: id}, nil
}

func (l *lock) Get(ctx context.Context) (*le.Record, []byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	w, err := l.repo.Worktree()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get worktree: %w", err)
	}
	if err := w.PullContext(ctx, &git.PullOptions{Auth: l.auth, Force: true}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		if errorContains(err, "remote repository is empty") {
			return nil, nil, apierrs.NewNotFound(schema.GroupResource{}, l.name)
		}
		return nil, nil, fmt.Errorf("failed to pull: %w", err)
	}
	f, err := w.Filesystem.Open(l.name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	r := &le.Record{}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, nil, fmt.Errorf("failed to decode: %w", err)
	}
	if err := w.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return nil, nil, fmt.Errorf("failed to clean: %w", err)
	}
	return r, b, nil
}

func (l *lock) Create(ctx context.Context, ler le.Record) error {
	return l.set(ctx, ler)
}

func (l *lock) Update(ctx context.Context, ler le.Record) error {
	return l.set(ctx, ler)
}

func (l *lock) RecordEvent(m string) {
	logrus.Infof("lock event: %s", m)
}

func (l *lock) Identity() string {
	return l.id
}

func (l *lock) Describe() string {
	return l.name
}

func (l *lock) set(ctx context.Context, ler le.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, err := l.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	h, err := l.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get head: %w", err)
	}
	f, err := w.Filesystem.Create(l.name)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	b, err := json.Marshal(ler)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	s, err := w.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	if s.IsClean() {
		return nil
	}
	if _, err := w.Add(l.name); err != nil {
		return fmt.Errorf("failed to add file: %w", err)
	}
	if _, err := w.Commit(fmt.Sprintf("%s lock", ler.HolderIdentity), &git.CommitOptions{}); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	if err := l.repo.PushContext(ctx, &git.PushOptions{Auth: l.auth}); err != nil {
		if err := w.Checkout(&git.CheckoutOptions{Hash: h.Hash()}); err != nil {
			return fmt.Errorf("failed to checkout: %w", err)
		}
		return fmt.Errorf("failed to push: %w", err)
	}
	return nil
}

func errorContains(err error, s string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), s)
}
