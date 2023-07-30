# Git backend for leader election

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/git.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/git)


## Usage

```go
package main

import (
  "context"
  "os"
  "time"

  gssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
  "github.com/sirupsen/logrus"
  "k8s.io/apimachinery/pkg/util/rand"

  le "go.linka.cloud/leaderelection"
  "go.linka.cloud/leaderelection/git"
)

const leaseName = "git-lock"

var (
  user    = os.Getenv("GIT_USER")
  keyPath = os.Getenv("GIT_KEY_PATH")
  repo    = os.Getenv("GIT_REPO")
)

func main() {
  ctx, cancel := context.WithCancel(le.SetupSignalHandler())
  defer cancel()

  auth, err := gssh.NewPublicKeysFromFile("git", keyPath, "")
  if err != nil {
    logrus.Fatal(err)
  }

  l, err := git.New(ctx, leaseName, repo, auth, rand.String(8))
  if err != nil {
    logrus.Fatal(err)
  }
  config := le.Config{
    Lock:            l,
    LeaseDuration:   15 * time.Second,
    RenewDeadline:   10 * time.Second,
    RetryPeriod:     5 * time.Second,
    ReleaseOnCancel: true,
    Name:            leaseName,
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
