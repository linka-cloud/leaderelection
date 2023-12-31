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
	"github.com/hashicorp/memberlist"
)

var _ memberlist.Broadcast = (*broadcast)(nil)

type broadcast struct {
	payload []byte
	action  *action
}

func (b *broadcast) Invalidates(o memberlist.Broadcast) bool {
	if ob, ok := o.(*broadcast); ok {
		return b.action.key == ob.action.key && b.action.typ == ob.action.typ
	}
	return false
}

func (b *broadcast) Message() []byte {
	return b.payload
}

func (b *broadcast) Finished() {}
