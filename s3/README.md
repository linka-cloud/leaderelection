# S3 backend for leader election

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/s3.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/s3)


## Usage


```go
package main

import (
  "context"
  "os"
  "time"

  "github.com/minio/minio-go/v7"
  "github.com/minio/minio-go/v7/pkg/credentials"
  "github.com/sirupsen/logrus"
  "k8s.io/apimachinery/pkg/util/rand"

  le "go.linka.cloud/leaderelection"
  "go.linka.cloud/leaderelection/s3"
)

const lockName = "test"

var (

  endpoint  = os.Getenv("S3_ENDPOINT")
  accessKey = os.Getenv("S3_ACCESS_KEY")
  secretKey = os.Getenv("S3_SECRET_KEY")

  bucket = os.Getenv("S3_BUCKET")
)

func main() {
  ctx, cancel := context.WithCancel(le.SetupSignalHandler())
  defer cancel()

  opts := &minio.Options{
    Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
    Secure: true,
  }
  l, err := s3.New(ctx, endpoint, bucket, "tests", "test", rand.String(8), opts)
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
