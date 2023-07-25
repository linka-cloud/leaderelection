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
	"encoding/binary"
	"time"

	"golang.org/x/sys/unix"
)

type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	Close() error
}

type kv struct {
	key       string
	time      time.Time
	value     []byte
	confirmed chan struct{}
}

func (k *kv) Encode() []byte {
	buff := bytes.NewBuffer(nil)
	buff.Write(must(unix.ByteSliceFromString(k.key)))
	if err := binary.Write(buff, binary.BigEndian, uint64(k.time.UnixMilli())); err != nil {
		panic(err)
	}
	if err := binary.Write(buff, binary.BigEndian, uint64(len(k.value))); err != nil {
		panic(err)
	}
	buff.Write(k.value)
	return buff.Bytes()
}

func (k *kv) Decode(buf []byte) (int, error) {
	if len(buf) < 1 {
		return 0, nil
	}
	k.key = unix.ByteSliceToString(buf[0:])
	k.time = time.UnixMilli(int64(binary.BigEndian.Uint64(buf[1+len(k.key):])))
	l := binary.BigEndian.Uint64(buf[1+8+len(k.key):])
	o := 1 + 8 + 8 + len(k.key)
	k.value = buf[o : o+int(l)]
	return o + int(l), nil
}
