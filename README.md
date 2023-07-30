# leaderelection

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection)

**Project status: *alpha***

*The API, spec, status and other user facing objects are subject to change.* 
*We do not support backward-compatibility for the alpha releases.*

## Overview

This module is extracted from the [k8s.io/client-go](https://github.com/kubernetes/client-go) package. 
It provides a simple interface for implementing leader election in a distributed system using different backends:
- [kubernetes](k8s): using [Lease](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#lease-v1-coordination-k8s-io) 
  (Kubernetes API)

  [![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/k8s.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/k8s)

- [gossip](gossip): using distributed in memory key-value store (based on [hashicorp/memberlist](https://github.com/hashicorp/memberlist))

  [![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/gossip.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/gossip)

- [s3](s3): using a simple json file stored in a s3 bucket

  [![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/s3.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/s3)

- [git](git): using a simple json file stored in a git repository
  
  [![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/git.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/git)


## Usage

```go
package main

import (
    "context"
    "time"
    
    "k8s.io/apimachinery/pkg/util/rand"
    
    le "go.linka.cloud/leaderelection"
    "go.linka.cloud/leaderelection/[backend]"
)

const (
	lockName = "test"
)

func main() {
	ctx, cancel := context.WithCancel(le.SetupSignalHandler())
	defer cancel()

	
	l, err := [backend].New(ctx, lockName, rand.String(8), ...)
	if err != nil {
		logrus.Fatal(err)
	}
	config := le.Config{
		Lock:            l,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		ReleaseOnCancel: true,
		Name:            lockName,
		Callbacks: le.Callbacks{
			OnStartedLeading: func(ctx context.Context) {
				logrus.Info("started leading")
			},
			OnStoppedLeading: func() {
				logrus.Info("stopped leading")
			},
			OnNewLeader: func(identity string) {
				logrus.Infof("new leader: %s", identity)
			},
		},
	}
	e, err := le.New(config)
	if err != nil {
		logrus.Fatal(err)
	}
	e.Run(ctx)
}
```
