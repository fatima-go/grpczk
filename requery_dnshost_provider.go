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
 * @date 25. 4. 2. 오후 5:23
 *
 */

package grpczk

import (
	"context"
	"fmt"
	"github.com/go-zookeeper/zk"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	_defaultLookupTimeout   = 3 * time.Second
	_defaultRequeryInterval = 5 * time.Second
)

type lookupHostFn func(context.Context, string) ([]string, error)

// RequeryDNSHostProviderOption is an option for the RequeryDNSHostProvider.
type RequeryDNSHostProviderOption interface {
	apply(*RequeryDNSHostProvider)
}

type lookupTimeoutOption struct {
	timeout time.Duration
}

// WithLookupTimeout returns a RequeryDNSHostProviderOption that sets the lookup timeout.
func WithLookupTimeout(timeout time.Duration) RequeryDNSHostProviderOption {
	if timeout < 0 {
		timeout = _defaultLookupTimeout // minimum
	}

	return lookupTimeoutOption{
		timeout: timeout,
	}
}

func (o lookupTimeoutOption) apply(provider *RequeryDNSHostProvider) {
	provider.lookupTimeout = o.timeout
}

type requeryIntervalOption struct {
	interval time.Duration
}

// WithLookupTimeout returns a RequeryDNSHostProviderOption that sets the lookup timeout.
func WithRequeryInterval(interval time.Duration) RequeryDNSHostProviderOption {
	if interval < time.Second {
		interval = time.Second // minimum
	}

	return requeryIntervalOption{
		interval: interval,
	}
}

func (o requeryIntervalOption) apply(provider *RequeryDNSHostProvider) {
	provider.requeryInterval = o.interval
}

// RequeryDNSHostProvider is the default HostProvider. It currently matches
// the Java StaticHostProvider, resolving hosts from DNS once during
// the call to Init.  It could be easily extended to re-query DNS
// periodically or if there is trouble connecting.
type RequeryDNSHostProvider struct {
	mu               sync.Mutex // Protects everything, so we can add asynchronous updates later.
	servers          []string
	curr             int
	last             int
	lookupTimeout    time.Duration
	requeryInterval  time.Duration
	dnsRequeryTicker *time.Ticker
	lookupHost       lookupHostFn // Override of net.LookupHost, for testing.
}

// NewRequeryDNSHostProvider creates a new RequeryDNSHostProvider with the given options.
func NewRequeryDNSHostProvider(options ...RequeryDNSHostProviderOption) *RequeryDNSHostProvider {
	var provider RequeryDNSHostProvider
	provider.lookupTimeout = _defaultLookupTimeout
	provider.requeryInterval = _defaultRequeryInterval
	for _, option := range options {
		option.apply(&provider)
	}
	return &provider
}

// Init is called first, with the servers specified in the connection
// string. It uses DNS to look up addresses for each server, then
// shuffles them all together.
func (hp *RequeryDNSHostProvider) Init(servers []string) error {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	zk.DefaultLogger.Printf("RequeryDNSHostProvider servers : %v", servers)
	found, err := hp.resolveServerAddressList(servers)
	if err != nil {
		return err
	}

	zk.DefaultLogger.Printf("RequeryDNSHostProvider.Init servers=%s", found)
	hp.servers = found
	hp.curr = -1
	hp.last = -1

	hp.dnsRequeryTicker = time.NewTicker(hp.requeryInterval)
	go func() {
		for range hp.dnsRequeryTicker.C {
			hp.lookupServers(servers)
		}
	}()

	return nil
}

func (hp *RequeryDNSHostProvider) resolveServerAddressList(servers []string) ([]string, error) {
	lookupHost := hp.lookupHost
	if lookupHost == nil {
		var resolver net.Resolver
		lookupHost = resolver.LookupHost
	}

	ctx, cancel := context.WithTimeout(context.Background(), hp.lookupTimeout)
	defer cancel()

	found := []string{}
	for _, server := range servers {
		host, port, err := net.SplitHostPort(server)
		if err != nil {
			return nil, err
		}
		addrs, err := lookupHost(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			found = append(found, net.JoinHostPort(addr, port))
		}
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("No hosts found for addresses %q", servers)
	}

	// Randomize the order of the servers to avoid creating hotspots
	stringShuffle(found)
	return found, nil
}

func (hp *RequeryDNSHostProvider) lookupServers(servers []string) {
	found, err := hp.resolveServerAddressList(servers)
	if err != nil {
		zk.DefaultLogger.Printf("resolveServerAddressList error. server=[%v] : %s", servers, err.Error())
		return
	}

	if !isAddressChanged(hp.servers, found) {
		return // same
	}

	hp.mu.Lock()
	defer hp.mu.Unlock()

	zk.DefaultLogger.Printf("RequeryDNSHostProvider.lookupServers servers=%s", found)
	hp.servers = found
	hp.curr = -1
	hp.last = -1
}

func isAddressChanged(origin, candidate []string) bool {
	for _, candidateAddr := range candidate {
		candidateHost := strings.Split(candidateAddr, ":")[0]
		exist := false
		for _, originAddr := range origin {
			originHost := strings.Split(originAddr, ":")[0]
			if originHost == candidateHost {
				exist = true
				break
			}
		}
		if !exist {
			// 후보 서버 IP가 단 1개라도 기존 서버 목록에 존재하지 않으면 IP 가 변경된걸로 판단한다
			return true
		}
	}

	return false
}

// Len returns the number of servers available
func (hp *RequeryDNSHostProvider) Len() int {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	return len(hp.servers)
}

// Next returns the next server to connect to. retryStart will be true
// if we've looped through all known servers without Connected() being
// called.
func (hp *RequeryDNSHostProvider) Next() (server string, retryStart bool) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.curr = (hp.curr + 1) % len(hp.servers)
	retryStart = hp.curr == hp.last
	if hp.last == -1 {
		hp.last = 0
	}
	return hp.servers[hp.curr], retryStart
}

// Connected notifies the HostProvider of a successful connection.
func (hp *RequeryDNSHostProvider) Connected() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.last = hp.curr
}

// stringShuffle performs a Fisher-Yates shuffle on a slice of strings
func stringShuffle(s []string) {
	for i := len(s) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
