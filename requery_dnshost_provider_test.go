/*
 * Copyright 2025 github.com/fatima-go
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @project fatima-core
 * @author dave_01
 * @date 25. 4. 2. 오후 6:10
 *
 */

package grpczk

import (
	"context"
	"github.com/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

const (
	_devZkHostList = "zk.dev.music-flo.io:2181,zk.dev.music-flo.io:2182,zk.dev.music-flo.io:2183"
)

var (
	_devZkAddressBefore = []string{"10.180.1.1", "10.180.1.2", "10.180.1.3"}
	_devZkAddressAfter  = []string{"10.180.200.1", "10.180.200.2", "10.180.200.3"}
)

var (
	dnsChanged = false
)

func TestNewRequeryDNSHostProvider(t *testing.T) {
	// func(context.Context, string) ([]string, error)
	dnsHostProvider := NewRequeryDNSHostProvider(
		withLookupHost(mockLookupHost),
		WithRequeryInterval(time.Second),
	)

	servers := zk.FormatServers(parseIpList(_devZkHostList))
	err := dnsHostProvider.Init(servers)
	if !assert.Nil(t, err) {
		t.Fatalf("failed to init : %s", err.Error())
		return
	}

	nextZkServerAddr, _ := dnsHostProvider.Next()
	assert.True(t, containAddr(_devZkAddressBefore, nextZkServerAddr))
	dnsChanged = true

	time.Sleep(2 * time.Second) // wait for refresh tick

	nextZkServerAddr, _ = dnsHostProvider.Next()
	assert.True(t, containAddr(_devZkAddressAfter, nextZkServerAddr))
}

func containAddr(list []string, addr string) bool {
	zk.DefaultLogger.Printf("containAddr : list=[%s], nextServer=%s", list, addr)
	host := strings.Split(addr, ":")
	for _, v := range list {
		if v == host[0] {
			return true
		}
	}
	return false
}

var (
	resolveIndex = 0
)

func mockLookupHost(ctx context.Context, host string) ([]string, error) {
	index := resolveIndex % 3
	if dnsChanged {
		return []string{_devZkAddressAfter[index]}, nil
	}
	return []string{_devZkAddressBefore[index]}, nil
}

type lookupHostOption struct {
	lookupFn lookupHostFn
}

func (o lookupHostOption) apply(provider *RequeryDNSHostProvider) {
	provider.lookupHost = o.lookupFn
}

func withLookupHost(lookupFn lookupHostFn) RequeryDNSHostProviderOption {
	return lookupHostOption{
		lookupFn: lookupFn,
	}
}
