package gossip_leaderelection

import (
	"github.com/hashicorp/memberlist"
)

var _ memberlist.Broadcast = (*broadcast)(nil)

type broadcast struct {
	payload []byte
}

func (b *broadcast) Invalidates(o memberlist.Broadcast) bool {
	return false
}

func (b *broadcast) Message() []byte {
	return b.payload
}

func (b *broadcast) Finished() {}
