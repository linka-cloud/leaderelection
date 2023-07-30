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

package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/minio/minio-go/v7"

	le "go.linka.cloud/leaderelection"
)

var _ le.Lock = (*lock)(nil)

func New(_ context.Context, endpoint, bucket, prefix, name, id string, opts *minio.Options) (le.Lock, error) {
	c, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s/%s.lock.json", prefix, name)
	return &lock{
		c:      c,
		id:     id,
		name:   name,
		bucket: bucket,
		key:    key,
	}, nil
}

type lock struct {
	mu sync.RWMutex
	c  *minio.Client

	id   string
	name string

	bucket string
	key    string

	etag string
}

func (l *lock) Get(ctx context.Context) (*le.Record, []byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	s, err := l.c.StatObject(ctx, l.bucket, l.key, minio.StatObjectOptions{})
	if isNotFound(err) {
		return nil, nil, fmt.Errorf("%s: %w", l.key, os.ErrNotExist)
	}
	l.etag = s.ETag

	o, err := l.c.GetObject(ctx, l.bucket, l.key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer o.Close()

	b, err := io.ReadAll(o)
	var ler le.Record
	if err := json.Unmarshal(b, &ler); err != nil {
		return nil, nil, err
	}
	return &ler, b, nil
}

func (l *lock) Create(ctx context.Context, ler le.Record) error {
	return l.set(ctx, ler)
}

func (l *lock) Update(ctx context.Context, ler le.Record) error {
	return l.set(ctx, ler)
}

func (l *lock) RecordEvent(_ string) {}

func (l *lock) Identity() string {
	return l.id
}

func (l *lock) Describe() string {
	return fmt.Sprintf("s3/%s", l.name)
}

func (l *lock) set(ctx context.Context, ler le.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, err := json.Marshal(ler)
	if err != nil {
		return err
	}

	s, err := l.c.StatObject(ctx, l.bucket, l.key, minio.StatObjectOptions{})
	if err != nil && !isNotFound(err) {
		return err
	}
	if s.ETag != l.etag {
		return fmt.Errorf("etag mismatch: %s != %s", s.ETag, l.etag)
	}

	o, err := l.c.PutObject(ctx, l.bucket, l.key, bytes.NewReader(b), int64(len(b)), minio.PutObjectOptions{ContentType: "application/json"})
	if err != nil {
		return err
	}
	l.etag = o.ETag
	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var e minio.ErrorResponse
	if errors.As(err, &e) {
		return e.StatusCode == 404
	}
	return false
}
