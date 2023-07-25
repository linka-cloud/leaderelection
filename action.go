package gossip_leaderelection

import (
	"bytes"

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
	key   string
	value []byte
}

func (a *action) Encode() []byte {
	buff := bytes.NewBuffer(nil)
	buff.Write([]byte{byte(a.typ)})
	buff.Write(must(unix.ByteSliceFromString(a.key)))
	buff.Write(a.value)
	return buff.Bytes()
}

func (a *action) Decode(buf []byte) error {
	if len(buf) < 1 {
		return nil
	}
	a.typ = actionType(buf[0])
	a.key = unix.ByteSliceToString(buf[1:])
	a.value = buf[2+len(a.key):]
	return nil
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
