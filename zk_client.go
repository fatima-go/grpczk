//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project throosea.com
// @author DeockJin Chung (jin.freestyle@gmail.com)
// @date 2017. 10. 1. PM 7:42
//

package grpczk

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/go-zookeeper/zk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
	"time"
)

const (
	grpcServiceConfig = `{"loadBalancingConfig": [{"round_robin":{}}]}` // This sets the initial balancing policy
)

var zm sync.Mutex
var zkClientServant *ZkClientServant

// NewZkClientServant return singleton ZkClientServant
func NewZkClientServant(zkIpList string) *ZkClientServant {
	if zkClientServant != nil {
		return zkClientServant
	}

	zm.Lock()
	defer zm.Unlock()

	// check one more
	if zkClientServant != nil {
		return zkClientServant
	}

	clientServant := &ZkClientServant{}
	clientServant.zkServant = NewZkServant(zkIpList)
	clientServant.serviceConfig = grpcServiceConfig // default round_robin config

	zkClientServant = clientServant
	return clientServant
}

type ZkClientServant struct {
	zkServant     *ZkServant
	serviceConfig string
	errorLogger   zk.Logger
}

func (z *ZkClientServant) SetLogger(logger zk.Logger) *ZkClientServant {
	z.zkServant.SetLogger(logger)
	return z
}

func (z *ZkClientServant) SetErrorLogger(logger zk.Logger) *ZkClientServant {
	z.zkServant.SetErrorLogger(logger)
	z.errorLogger = logger
	return z
}

func (z *ZkClientServant) SetDebug(debug bool) *ZkClientServant {
	z.zkServant.SetDebug(debug)
	return z
}

func (z *ZkClientServant) Disconnect(znodePath string) {
	zm.Lock()
	defer zm.Unlock()

	unregistConnectionHelper(znodePath)
	unregistServiceResolver(znodePath)

	if isEmptyResolveMap() {
		zk.DefaultLogger.Printf("closing zkServant...")
		retireZkClientServant := zkClientServant
		zkClientServant = nil
		retireZkClientServant.zkServant.Close()
	}
}

func (z *ZkClientServant) ConnectWithHelper(znodePath string, zkHelper ZKConnectionHelper, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if zkHelper != nil {
		registConnectionHelper(znodePath, zkHelper)
		balancerName := zkHelper.GetBalancerName()
		if len(balancerName) > 0 {
			zk.DefaultLogger.Printf("using balancer %s", balancerName)
			z.serviceConfig = fmt.Sprintf(`{"loadBalancingConfig": [{"%s":{}}]}`, balancerName)
		}
	}
	return z.Connect(znodePath, opts...)
}

func (z *ZkClientServant) Connect(znodePath string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	err := z.zkServant.Connect()
	if err != nil {
		return nil, err
	}

	mode, err := z.zkServant.GetTransportMode(znodePath)
	if err != nil {
		return nil, err
	}
	if mode == TransportModeUnknown {
		return nil, fmt.Errorf("unknown transport mode in service description : %s", znodePath)
	}

	children, ch, err := z.zkServant.ChildrenW(znodePath)
	if err != nil {
		return nil, err
	}

	if len(children) == 0 {
		return nil, fmt.Errorf("%s : there is no server", znodePath)
	}

	zk.DefaultLogger.Printf("[%s] initial server list : %v", znodePath, children)

	registServiceResolver(znodePath, children)

	grpc.EnableTracing = false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	dialTarget := fmt.Sprintf("%s:///%s", grpczkScheme, znodePath)

	dialOpts := make([]grpc.DialOption, 0)
	dialOpts = append(dialOpts, grpc.WithBlock())
	dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(z.serviceConfig))
	if len(opts) > 0 {
		dialOpts = append(dialOpts, opts...)
	}

	var gConn *grpc.ClientConn
	switch mode {
	case TransportModePlain:
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	case TransportModeSsl:
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}
		creds := credentials.NewTLS(conf)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	default:
		return nil, fmt.Errorf("invalid transport mode : %v", mode)
	}

	gConn, err = grpc.DialContext(
		ctx,
		dialTarget,
		dialOpts...,
	)

	if err == nil {
		// start watch node...
		go func() {
			z.watchNode(znodePath, children, ch)
		}()
	}

	return gConn, err
}

func (z *ZkClientServant) GetData(znodePath string) ([]byte, error) {
	return z.zkServant.GetData(znodePath)
}

func (z *ZkClientServant) SetData(znodePath string, data []byte) error {
	if data == nil {
		return nil
	}

	return z.zkServant.SetData(znodePath, data)
}

func (z *ZkClientServant) watchNode(znodePath string, children []string, ch <-chan zk.Event) {
	var err error

	defer func() {
		zk.DefaultLogger.Printf("[%s] stop watching node", znodePath)
	}()

	for {
		e := <-ch
		children, ch, err = z.zkServant.ChildrenW(znodePath)
		if err != nil {
			zk.DefaultLogger.Printf("[%s] zk error : %s", znodePath, err.Error())
			// TODO : znode watch만 다시 하면 될거 같은데...
			z.zkServant.Close()

			// 감시 해야 할 znode path 가 존재하지 않는다면 리턴
			if !hasServiceResolver(znodePath) {
				return
			}

			err = z.zkServant.Connect()
			time.Sleep(time.Second * 5)
			continue
		}

		if e.Type&zk.EventNodeChildrenChanged != zk.EventNodeChildrenChanged {
			continue
		}

		// 연결이 종료되었을 경우 updateServerList 호출을 skip
		if e.State == zk.StateDisconnected {
			continue
		}

		err = updateServerList(znodePath, children)
		if err != nil {
			if z.errorLogger != nil {
				z.errorLogger.Printf("[%s] fail to update service list [addr.len=%d] : %s", znodePath, len(children), err.Error())
			}
			zk.DefaultLogger.Printf("[%s] fail to update service list : %s. children=%v", znodePath, err.Error(), children)
			if err == errNotfoundServiceName {
				return
			}
		}
	}
}
