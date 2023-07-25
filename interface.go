/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package leaderelection

import (
	"context"
)

// Record is the record that is stored in the leader election annotation.
// This information should be used for observational purposes only and could be replaced
// with a random string (e.g. UUID) with only slight modification of this code.
// TODO(mikedanese): this should potentially be versioned
type Record struct {
	// HolderIdentity is the ID that owns the lease. If empty, no one owns this lease and
	// all callers may acquire. Versions of this library prior to Kubernetes 1.14 will not
	// attempt to acquire leases with empty identities and will wait for the full lease
	// interval to expire before attempting to reacquire. This value is set to empty when
	// a client voluntarily steps down.
	HolderIdentity            string `json:"holderIdentity"`
	LeaseDurationMilliSeconds int    `json:"leaseDurationMilliSeconds"`
	AcquireTime               int64  `json:"acquireTime"`
	RenewTime                 int64  `json:"renewTime"`
	LeaderTransitions         int    `json:"leaderTransitions"`
}

// Lock offers a common interface for locking on arbitrary
// resources used in leader election.  The Lock is used
// to hide the details on specific implementations in order to allow
// them to change over time.  This interface is strictly for use
// by the leaderelection code.
type Lock interface {
	// Get returns the Record
	Get(ctx context.Context) (*Record, []byte, error)

	// Create attempts to create a Record
	Create(ctx context.Context, ler Record) error

	// Update will update and existing Record
	Update(ctx context.Context, ler Record) error

	// RecordEvent is used to record events
	RecordEvent(string)

	// Identity will return the locks Identity
	Identity() string

	// Describe is used to convert details on current resource lock
	// into a string
	Describe() string
}
