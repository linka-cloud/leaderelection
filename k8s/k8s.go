/*
Copyright 2018 The Kubernetes Authors.

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

package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	restclient "k8s.io/client-go/rest"

	le "go.linka.cloud/leaderelection"
)

// EventRecorder records a change in the ResourceLock.
type EventRecorder interface {
	Eventf(obj runtime.Object, eventType, reason, message string, args ...interface{})
}

// Config common data that exists across different
// resource locks
type Config struct {
	// Identity is the unique string identifying a lease holder across
	// all participants in an election.
	Identity string
	// EventRecorder is optional.
	EventRecorder EventRecorder
}

// New will create a lock of a given type according to the input parameters
func New(ns string, name string, coordinationClient coordinationv1client.CoordinationV1Interface, rlc Config) (le.Lock, error) {
	return &LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Client:     coordinationClient,
		LockConfig: rlc,
	}, nil
}

// NewFromKubeconfig will create a lock of a given type according to the input parameters.
// Timeout set for a client used to contact to Kubernetes should be lower than
// RenewDeadline to keep a single hung request from forcing a leader loss.
// Setting it to max(time.Second, RenewDeadline/2) as a reasonable heuristic.
func NewFromKubeconfig(ns string, name string, rlc Config, kubeconfig *restclient.Config, renewDeadline time.Duration) (le.Lock, error) {
	// shallow copy, do not modify the kubeconfig
	config := *kubeconfig
	timeout := renewDeadline / 2
	if timeout < time.Second {
		timeout = time.Second
	}
	config.Timeout = timeout
	leaderElectionClient := clientset.NewForConfigOrDie(restclient.AddUserAgent(&config, "leader-election"))
	return New(ns, name, leaderElectionClient.CoordinationV1(), rlc)
}

type LeaseLock struct {
	// LeaseMeta should contain a Name and a Namespace of a
	// LeaseMeta object that the LeaderElector will attempt to lead.
	LeaseMeta  metav1.ObjectMeta
	Client     coordinationv1client.LeasesGetter
	LockConfig Config
	lease      *coordinationv1.Lease
}

// Get returns the election record from a Lease spec
func (ll *LeaseLock) Get(ctx context.Context) (*le.Record, []byte, error) {
	lease, err := ll.Client.Leases(ll.LeaseMeta.Namespace).Get(ctx, ll.LeaseMeta.Name, metav1.GetOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, nil, os.ErrNotExist
		}
		return nil, nil, err
	}
	ll.lease = lease
	record := LeaseSpecToLeaderElectionRecord(&ll.lease.Spec)
	recordByte, err := json.Marshal(*record)
	if err != nil {
		return nil, nil, err
	}
	return record, recordByte, nil
}

// Create attempts to create a Lease
func (ll *LeaseLock) Create(ctx context.Context, ler le.Record) error {
	var err error
	ll.lease, err = ll.Client.Leases(ll.LeaseMeta.Namespace).Create(ctx, &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ll.LeaseMeta.Name,
			Namespace: ll.LeaseMeta.Namespace,
		},
		Spec: LeaderElectionRecordToLeaseSpec(&ler),
	}, metav1.CreateOptions{})
	return err
}

// Update will update an existing Lease spec.
func (ll *LeaseLock) Update(ctx context.Context, ler le.Record) error {
	if ll.lease == nil {
		return errors.New("lease not initialized, call get or create first")
	}
	ll.lease.Spec = LeaderElectionRecordToLeaseSpec(&ler)

	lease, err := ll.Client.Leases(ll.LeaseMeta.Namespace).Update(ctx, ll.lease, metav1.UpdateOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return os.ErrNotExist
		}
		return err
	}

	ll.lease = lease
	return nil
}

// RecordEvent in leader election while adding meta-data
func (ll *LeaseLock) RecordEvent(s string) {
	if ll.LockConfig.EventRecorder == nil {
		return
	}
	events := fmt.Sprintf("%v %v", ll.LockConfig.Identity, s)
	subject := &coordinationv1.Lease{ObjectMeta: ll.lease.ObjectMeta}
	// Populate the type meta, so we don't have to get it from the schema
	subject.Kind = "Lease"
	subject.APIVersion = coordinationv1.SchemeGroupVersion.String()
	ll.LockConfig.EventRecorder.Eventf(subject, corev1.EventTypeNormal, "LeaderElection", events)
}

// Describe is used to convert details on current resource lock
// into a string
func (ll *LeaseLock) Describe() string {
	return fmt.Sprintf("%v/%v", ll.LeaseMeta.Namespace, ll.LeaseMeta.Name)
}

// Identity returns the Identity of the lock
func (ll *LeaseLock) Identity() string {
	return ll.LockConfig.Identity
}

func LeaseSpecToLeaderElectionRecord(spec *coordinationv1.LeaseSpec) *le.Record {
	var r le.Record
	if spec.HolderIdentity != nil {
		r.HolderIdentity = *spec.HolderIdentity
	}
	if spec.LeaseDurationSeconds != nil {
		r.LeaseDurationMilliSeconds = int(*spec.LeaseDurationSeconds) * 1000
	}
	if spec.LeaseTransitions != nil {
		r.LeaderTransitions = int(*spec.LeaseTransitions)
	}
	if spec.AcquireTime != nil {
		r.AcquireTime = spec.AcquireTime.Time.UnixMilli()
	}
	if spec.RenewTime != nil {
		r.RenewTime = spec.RenewTime.Time.UnixMilli()
	}
	return &r

}

func LeaderElectionRecordToLeaseSpec(ler *le.Record) coordinationv1.LeaseSpec {
	leaseDurationSeconds := int32(ler.LeaseDurationMilliSeconds) * 1000
	leaseTransitions := int32(ler.LeaderTransitions)
	return coordinationv1.LeaseSpec{
		HolderIdentity:       &ler.HolderIdentity,
		LeaseDurationSeconds: &leaseDurationSeconds,
		AcquireTime:          &metav1.MicroTime{Time: time.UnixMilli(ler.AcquireTime)},
		RenewTime:            &metav1.MicroTime{Time: time.UnixMilli(ler.RenewTime)},
		LeaseTransitions:     &leaseTransitions,
	}
}
