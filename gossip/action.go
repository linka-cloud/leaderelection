package gossip

import (
	"bytes"
	"encoding/binary"
	"time"

	"golang.org/x/sys/unix"
)

const (
	actionTypeUnknown actionType = iota
	actionTypeSet
	actionTypeDelete
)

type actionType uint8

func (b actionType) String() string {
	switch b {
	case actionTypeSet:
		return "set"
	case actionTypeDelete:
		return "delete"
	default:
		return "unknown"
	}
}

type action struct {
	typ   actionType
	time  time.Time
	key   string
	value []byte
}

func (a *action) Encode() []byte {
	buff := bytes.NewBuffer(nil)
	buff.Write([]byte{byte(a.typ)})
	if err := binary.Write(buff, binary.LittleEndian, uint64(a.time.UnixMilli())); err != nil {
		panic(err)
	}
	buff.Write(must(unix.ByteSliceFromString(a.key)))
	buff.Write(a.value)
	return buff.Bytes()
}

func (a *action) Decode(buf []byte) error {
	if len(buf) < 1 {
		return nil
	}
	a.typ = actionType(buf[0])
	a.time = time.UnixMilli(int64(binary.LittleEndian.Uint64(buf[1:])))
	a.key = unix.ByteSliceToString(buf[9:])
	a.value = buf[9+1+len(a.key):]
	return nil
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
