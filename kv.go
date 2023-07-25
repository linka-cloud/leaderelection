package gossip_leaderelection

import (
	"bytes"
	"context"
	"encoding/binary"

	"golang.org/x/sys/unix"
)

type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error

	Close() error
}

type kv struct {
	key   string
	value []byte
}

func (k *kv) Encode() []byte {
	buff := bytes.NewBuffer(nil)
	buff.Write(must(unix.ByteSliceFromString(k.key)))
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
	l := binary.BigEndian.Uint64(buf[1+len(k.key):])
	o := 9 + len(k.key)
	k.value = buf[o : o+int(l)]
	return o + int(l), nil
}
