// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package dns

import (
	"context"
	"net"
	"strings"
	"sync"

	"go.linka.cloud/grpc-toolkit/logger"
	"go.uber.org/multierr"

	"go.linka.cloud/leaderelection/gossip/internal/dns/godns"
	"go.linka.cloud/leaderelection/gossip/internal/dns/miekgdns"
)

// Provider is a stateful cache for asynchronous DNS resolutions. It provides a way to resolve addresses and obtain them.
type Provider struct {
	sync.RWMutex
	resolver Resolver
	// A map from domain name to a slice of resolved targets.
	resolved map[string][]string
	logger   logger.Logger
}

type ResolverType string

const (
	GolangResolverType   ResolverType = "golang"
	MiekgdnsResolverType ResolverType = "miekgdns"
)

func (t ResolverType) ToResolver(ctx context.Context) ipLookupResolver {
	var r ipLookupResolver
	switch t {
	case GolangResolverType:
		r = &godns.Resolver{Resolver: net.DefaultResolver}
	case MiekgdnsResolverType:
		r = &miekgdns.Resolver{ResolvConf: miekgdns.DefaultResolvConfPath}
	default:
		logger.C(ctx).WithField("type", t).Warn("no such resolver type, defaulting to golang")
		r = &godns.Resolver{Resolver: net.DefaultResolver}
	}
	return r
}

// NewProvider returns a new empty provider with a given resolver type.
// If empty resolver type is net.DefaultResolver.
func NewProvider(ctx context.Context, resolverType ResolverType) *Provider {
	p := &Provider{
		resolver: NewResolver(ctx, resolverType.ToResolver(ctx)),
		resolved: make(map[string][]string),
		logger:   logger.C(ctx),
	}

	return p
}

// Clone returns a new provider from an existing one.
func (p *Provider) Clone() *Provider {
	return &Provider{
		resolver: p.resolver,
		resolved: make(map[string][]string),
		logger:   p.logger,
	}
}

// IsDynamicNode returns if the specified StoreAPI addr uses
// any kind of SD mechanism.
func IsDynamicNode(addr string) bool {
	qtype, _ := GetQTypeName(addr)
	return qtype != ""
}

// GetQTypeName splits the provided addr into two parts: the QType (if any)
// and the name.
func GetQTypeName(addr string) (qtype, name string) {
	qtypeAndName := strings.SplitN(addr, "+", 2)
	if len(qtypeAndName) != 2 {
		return "", addr
	}
	return qtypeAndName[0], qtypeAndName[1]
}

// Resolve stores a list of provided addresses or their DNS records if requested.
// Addresses prefixed with `dns+` or `dnssrv+` will be resolved through respective DNS lookup (A/AAAA or SRV).
// For non-SRV records, it will return an error if a port is not supplied.
func (p *Provider) Resolve(ctx context.Context, addrs []string) error {
	resolvedAddrs := map[string][]string{}
	var errs error

	for _, addr := range addrs {
		var resolved []string
		qtype, name := GetQTypeName(addr)
		if qtype == "" {
			resolvedAddrs[name] = []string{name}
			continue
		}

		resolved, err := p.resolver.Resolve(ctx, name, QType(qtype))
		if err != nil {
			// Use cached values.
			p.RLock()
			resolved = p.resolved[addr]
			p.RUnlock()
			errs = multierr.Append(errs, err)
		}
		resolvedAddrs[addr] = resolved
	}

	// All addresses have been resolved. We can now take an exclusive lock to
	// update the resolved addresses metric and update the local state.
	p.Lock()
	defer p.Unlock()

	p.resolved = resolvedAddrs

	return errs
}

// Addresses returns the latest addresses present in the Provider.
func (p *Provider) Addresses() []string {
	p.RLock()
	defer p.RUnlock()

	var result []string
	for _, addrs := range p.resolved {
		result = append(result, addrs...)
	}
	return result
}
