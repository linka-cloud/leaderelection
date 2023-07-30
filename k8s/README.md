# Kubernetes backend for leader election

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/k8s.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/k8s)


## Usage

```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"

	le "go.linka.cloud/leaderelection"
	"go.linka.cloud/leaderelection/k8s"
)

const (
	lockName = "test"
)

func main() {
	ctx, cancel := context.WithCancel(le.SetupSignalHandler())
	defer cancel()

	name := rand.String(8)
	path := os.Getenv("KUBECONFIG")
	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		logrus.Fatal(err)
	}
	rlc := k8s.Config{
		Identity: name,
	}
	l, err := k8s.NewFromKubeconfig("default", lockName, rlc, cfg, 10*time.Second)
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
