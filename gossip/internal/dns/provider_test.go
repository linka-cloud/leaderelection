// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package dns

import (
	"context"
	"sort"
	"testing"

	"github.com/efficientgo/core/testutil"
)

func TestProvider(t *testing.T) {
	ctx := context.TODO()
	ips := []string{
		"127.0.0.1:19091",
		"127.0.0.2:19092",
		"127.0.0.3:19093",
		"127.0.0.4:19094",
		"127.0.0.5:19095",
	}

	prv := NewProvider(ctx, "")
	prv.resolver = &mockResolver{
		res: map[string][]string{
			"a": ips[:2],
			"b": ips[2:4],
			"c": {ips[4]},
		},
	}

	err := prv.Resolve(ctx, []string{"any+x"})
	testutil.Ok(t, err)
	result := prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, []string(nil), result)

	err = prv.Resolve(ctx, []string{"any+a", "any+b", "any+c"})
	testutil.Ok(t, err)
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips, result)

	err = prv.Resolve(ctx, []string{"any+b", "any+c"})
	testutil.Ok(t, err)
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips[2:], result)

	err = prv.Resolve(ctx, []string{"any+x"})
	testutil.Ok(t, err)
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, []string(nil), result)

	err = prv.Resolve(ctx, []string{"any+a", "any+b", "any+c"})
	testutil.Ok(t, err)
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips, result)

	err = prv.Resolve(ctx, []string{"any+b", "example.com:90", "any+c"})
	testutil.Ok(t, err)
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, append(ips[2:], "example.com:90"), result)

	err = prv.Resolve(ctx, []string{"any+b", "any+c"})
	testutil.Ok(t, err)
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips[2:], result)

}

type mockResolver struct {
	res map[string][]string
	err error
}

func (d *mockResolver) Resolve(_ context.Context, name string, _ QType) ([]string, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.res[name], nil
}

// TestIsDynamicNode tests whether we properly catch dynamically defined nodes.
func TestIsDynamicNode(t *testing.T) {
	for _, tcase := range []struct {
		node      string
		isDynamic bool
	}{
		{
			node:      "1.2.3.4",
			isDynamic: false,
		},
		{
			node:      "gibberish+1.1.1.1+noa",
			isDynamic: true,
		},
		{
			node:      "",
			isDynamic: false,
		},
		{
			node:      "dns+aaa",
			isDynamic: true,
		},
		{
			node:      "dnssrv+asdasdsa",
			isDynamic: true,
		},
	} {
		isDynamic := IsDynamicNode(tcase.node)
		testutil.Equals(t, tcase.isDynamic, isDynamic, "mismatch between results")
	}
}
