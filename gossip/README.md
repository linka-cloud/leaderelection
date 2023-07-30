# Gossip backend for leader election

[![Language: Go](https://img.shields.io/badge/lang-Go-6ad7e5.svg?style=flat-square&logo=go)](https://golang.org/)
[![Go Reference](https://pkg.go.dev/badge/go.linka.cloud/leaderelection/gossip.svg)](https://pkg.go.dev/go.linka.cloud/leaderelection/gossip)


## Usage

```go
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"

	le "go.linka.cloud/leaderelection"
	"go.linka.cloud/leaderelection/gossip"
)

const (
	lockName = "test"
	pass     = "noop"
)

func main() {
	ctx, cancel := context.WithCancel(le.SetupSignalHandler())
	defer cancel()

	var (
		port  int
		addrs string
		lvl   string
	)

	flag.IntVar(&port, "port", 18888, "port")
	flag.StringVar(&addrs, "addrs", "localhost:18887,localhost:18888,localhost:18889", "memberlist addresses")
	flag.StringVar(&lvl, "log", "info", "log level")
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	logrus.StandardLogger().Formatter.(*logrus.TextFormatter).FullTimestamp = true
	logrus.StandardLogger().Formatter.(*logrus.TextFormatter).TimestampFormat = time.RFC3339Nano

	if lvl, err := logrus.ParseLevel(lvl); err == nil {
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.TraceLevel)
	}

	name, err := os.Hostname()
	if err != nil {
		logrus.Fatal(err)
	}
	name = fmt.Sprintf("%s-%d", name, port)

	tick := 100 * time.Millisecond

	c := memberlist.DefaultLocalConfig()
	c.Name = name
	c.BindPort = port
	c.RetransmitMult = len(addrs)
	c.PushPullInterval = 0
	c.SecretKey = hash(pass)
	c.GossipVerifyIncoming = true
	c.GossipVerifyOutgoing = true
	c.DisableTcpPings = true
	c.GossipInterval = tick
	l, err := gossip.New(ctx, c, lockName, name, strings.Split(addrs, ",")...)
	if err != nil {
		logrus.Fatal(err)
	}
	defer l.Close()
	
	config := le.Config{
		Name:            lockName,
		Lock:            l,
		LeaseDuration:   15 * tick,
		RenewDeadline:   10 * tick,
		RetryPeriod:     2 * tick,
		ReleaseOnCancel: true,
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

func hash(pass string) []byte {
	h := sha256.New()
	h.Write([]byte(pass))
	return h.Sum(nil)
}
```
