/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 * @project grpczk
 * @author dave
 * @date 22. 2. 25. 오전 1:29
 */

package grpczk

import (
	"fmt"
	"github.com/go-zookeeper/zk"
	"google.golang.org/grpc/resolver"
	"sync"
)

const (
	grpczkScheme       = "grpczk"
	exampleServiceName = "lb.example.grpc.io"
)

// resolveMap
// key : grpcServiceName(znodepath), value : grpczkResolver
var resolveMap map[string]ServerListUpdater

type ServerListUpdater interface {
	SetConnection(resolver.ClientConn)
	UpdateServerList([]string) error
	resolver.Resolver
}

var rmu sync.Mutex

func isEmptyResolveMap() bool {
	return len(resolveMap) == 0
}

func registServiceResolver(serviceName string, initialServerList []string) {
	rmu.Lock()
	defer rmu.Unlock()

	r := &grpczkResolver{}
	r.serviceName = serviceName
	r.initialAddrList = initialServerList
	resolveMap[serviceName] = r
	zk.DefaultLogger.Printf("resolver registed %s", serviceName)
}

func unregistServiceResolver(serviceName string) {
	rmu.Lock()
	defer rmu.Unlock()
	resolveMap[serviceName] = nil
	zk.DefaultLogger.Printf("resolver unregisted %s", serviceName)
}

var (
	errNotfoundServiceName = fmt.Errorf("not found service name while resolving")
)

func updateServerList(serviceName string, newServerList []string) error {
	rmu.Lock()
	defer rmu.Unlock()

	updater, ok := resolveMap[serviceName]
	if !ok {
		return errNotfoundServiceName
	}

	return updater.UpdateServerList(newServerList)
}

func init() {
	resolveMap = make(map[string]ServerListUpdater)
	resolver.Register(&floGrpcResolverBuilder{})
}

type floGrpcResolverBuilder struct{}

// Build creates a new resolver for the given target.
// gRPC dial calls Build synchronously, and fails if the returned error is not nil.
func (*floGrpcResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	zk.DefaultLogger.Printf("resolving [%s:/%s]", target.URL.Scheme, target.URL.Path)
	serviceName := getEndpoint(target)
	updater, ok := resolveMap[serviceName]
	if !ok {
		return nil, fmt.Errorf("not found resolver for %s", serviceName)
	}

	updater.SetConnection(cc)
	return updater, nil
}

// Scheme returns the scheme supported by this resolver.
// Scheme is defined at https://github.com/grpc/grpc/blob/master/doc/naming.md.
func (*floGrpcResolverBuilder) Scheme() string { return grpczkScheme }

type grpczkResolver struct {
	serviceName     string
	initialAddrList []string
	cc              resolver.ClientConn
}

func (r *grpczkResolver) SetConnection(cc resolver.ClientConn) {
	r.cc = cc
	r.UpdateServerList(r.initialAddrList)
}

func (r *grpczkResolver) UpdateServerList(addrList []string) error {
	if r.cc == nil {
		return fmt.Errorf("%s has no ClientConn", r.serviceName)
	}

	zk.DefaultLogger.Printf("%s update server list : %v", r.serviceName, addrList)
	newAddrList := make([]resolver.Address, len(addrList))
	for i, s := range addrList {
		newAddrList[i] = resolver.Address{Addr: s}
	}
	return r.cc.UpdateState(resolver.State{Addresses: newAddrList})
}

func (*grpczkResolver) ResolveNow(o resolver.ResolveNowOptions) {}
func (*grpczkResolver) Close()                                  {}

// remove leading slash
func getEndpoint(target resolver.Target) string {
	path := target.URL.Path
	if len(path) == 0 {
		return path
	}

	if path[0] == '/' {
		return path[1:]
	}

	return path
}
