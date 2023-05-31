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
	"encoding/json"
	"fmt"
	"github.com/go-zookeeper/zk"
	"reflect"
	"strings"
	"sync"
	"time"
)

// logging preference delivery mode
const (
	TransportModeUnknown = iota
	TransportModePlain
	TransportModeSsl
)

type TransportMode int

const (
	zkVersionAll        = -1
	ServiceKeyTransport = "transport"
)

type ZkServant struct {
	ipList           []string
	zkConn           *zk.Conn
	mutex            sync.Mutex
	cond             *sync.Cond
	state            zk.State
	errorLogger      zk.Logger
	debugging        bool
	pathSet          map[string]struct{}
	sessionAvailable bool
}

func NewZkServant(zkIpList string) *ZkServant {
	servant := ZkServant{}
	servant.ipList = parseIpList(zkIpList)
	servant.mutex = sync.Mutex{}
	servant.cond = sync.NewCond(&servant.mutex)
	servant.state = zk.StateDisconnected
	servant.pathSet = map[string]struct{}{}
	servant.sessionAvailable = true
	servant.debugging = true
	return &servant
}

func (z *ZkServant) SetLogger(logger zk.Logger) *ZkServant {
	zk.DefaultLogger = logger
	return z
}

func (z *ZkServant) SetErrorLogger(logger zk.Logger) *ZkServant {
	z.errorLogger = logger
	return z
}

func (z *ZkServant) SetDebug(debug bool) *ZkServant {
	z.debugging = debug
	return z
}

func (z *ZkServant) Connect() error {
	if z.zkConn != nil {
		return nil
	}

	//(*Conn, <-chan Event, error)
	var conn *zk.Conn
	var eventChan <-chan zk.Event
	var err error
	if z.debugging {
		conn, eventChan, err = zk.Connect(z.ipList, time.Second)
	} else {
		conn, eventChan, err = zk.Connect(z.ipList, time.Second, zk.WithLogInfo(false))
	}
	if err != nil {
		return err
	}
	z.zkConn = conn

	go func() {
		for elem := range eventChan {
			if !z.processZkEvent(elem) {
				return
			}
		}
	}()

	return nil
}

func (z *ZkServant) Close() {
	z.zkConn.Close()
	z.zkConn = nil
	z.sessionAvailable = false
	z.state = zk.StateDisconnected
	z.pathSet = nil
}

func (z *ZkServant) processZkEvent(event zk.Event) bool {
	if event.Type != zk.EventSession {
		return true
	}

	zk.DefaultLogger.Printf("zk event : %v", event)

	// report error for changing state
	if z.state != event.State && z.errorLogger != nil {
		switch event.State {
		case zk.StateConnectedReadOnly:
		case zk.StateSaslAuthenticated:
		case zk.StateConnecting:
		case zk.StateConnected:
		case zk.StateHasSession:
		default:
			z.errorLogger.Printf("zk status changed : %v", event)
		}
	}

	z.state = event.State

	z.cond.Broadcast()

	// StateUnknown, StateDisconnected, StateExpired, StateAuthFailed
	switch event.State {
	case zk.StateHasSession:
		zk.DefaultLogger.Printf("zk has session")
		if !z.sessionAvailable {
			z.reCreateAllEphemerals()
			z.sessionAvailable = true
		}
	case zk.StateConnected:
		zk.DefaultLogger.Printf("zk connected")
	case zk.StateAuthFailed:
		zk.DefaultLogger.Printf("zk auth failed")
	case zk.StateDisconnected:
		zk.DefaultLogger.Printf("zk disconnected")
	case zk.StateExpired:
		zk.DefaultLogger.Printf("zk expired")
		z.zkConn.Close()
		z.zkConn = nil
		z.sessionAvailable = false
		z.Connect()
		return false
	}

	return true
}

func (z *ZkServant) reCreateAllEphemerals() {
	for k, _ := range z.pathSet {
		err := z.createEphemeralOnce(k)
		if err != nil {
			zk.DefaultLogger.Printf("zk createEphemeralOnce fail : [%s] %s", k, err.Error())
		}
	}
}

func (z *ZkServant) ensureZkEnabled() {
	if z.state == zk.StateHasSession {
		return
	}

	z.mutex.Lock()
	defer z.mutex.Unlock()
	for z.state != zk.StateHasSession {
		z.cond.Wait()
	}
}

func (z *ZkServant) CreateEphemeral(path string) error {
	zk.DefaultLogger.Printf("CreateEphemeral : %s", path)
	z.ensureZkEnabled()

	z.pathSet[path] = struct{}{}
	acl := zk.WorldACL(zk.PermAll)
	for {
		_, err := z.zkConn.Create(path, nil, zk.FlagEphemeral, acl)
		if err != nil {
			zk.DefaultLogger.Printf("zk fail to createEphemeral : [%s] %s", path, err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}

	zk.DefaultLogger.Printf("zk create ephemeral path : %s", path)
	return nil
}

func (z *ZkServant) createEphemeralOnce(path string) error {
	zk.DefaultLogger.Printf("createEphemeralOnce : %s", path)
	z.ensureZkEnabled()

	z.pathSet[path] = struct{}{}
	acl := zk.WorldACL(zk.PermAll)
	_, err := z.zkConn.Create(path, nil, zk.FlagEphemeral, acl)
	if err != nil {
		return err
	}

	zk.DefaultLogger.Printf("zk create ephemeral path : %s", path)
	return nil
}

func (z *ZkServant) SetData(path string, data []byte) error {
	zk.DefaultLogger.Printf("zk setData [%s] : %d bytes", path, len(data))
	z.ensureZkEnabled()
	_, err := z.zkConn.Set(path, data, zkVersionAll)
	if err == zk.ErrNoNode {
		zk.DefaultLogger.Printf("[%s] node does not exist", path)
		err = z.ensurePath(path)
		if err != nil {
			return err
		}
		// set again
		_, err = z.zkConn.Set(path, data, zkVersionAll)
	}
	return err
}

type ZNodeDataHandleFunc func(znodeData []byte) ([]byte, error)

func (z *ZkServant) SetDataWithVersion(path string, handleFunc ZNodeDataHandleFunc) error {
	zk.DefaultLogger.Printf("zk SetDataWithVersion [%s]", path)

	delayUnit := time.Millisecond * 5
	delayMaximum := time.Millisecond * 500
	tryCount := 0

	for true {
		previous, stat, err := z.GetDataWithStat(path)
		if err != nil {
			return fmt.Errorf("SetDataWithVersion.GetDataWithStat : %s", err.Error())
		}

		converted, err := handleFunc(previous)
		if err != nil {
			return fmt.Errorf("handle func error : %s", err.Error())
		}

		_, err = z.zkConn.Set(path, converted, stat.Version)
		if err == nil {
			break
		}

		if err == zk.ErrNoNode {
			zk.DefaultLogger.Printf("[%s] node does not exist", path)
			err = z.ensurePath(path)
			if err != nil {
				return err
			}
			// set again
			_, err = z.zkConn.Set(path, converted, zkVersionAll)
			return err
		} else if err == zk.ErrBadVersion {
			zk.DefaultLogger.Printf("zk: version conflict [%s]", path)
			if tryCount > MaxSetDataTryCount {
				// 5, 10, 20, 40, 80, 160, 320, 500, 500, 500
				return fmt.Errorf("fail set to data. exceed max retry %s", err.Error())
			}
			time.Sleep(delayUnit)
			delayUnit = delayUnit * 2
			if delayUnit > delayMaximum {
				delayUnit = delayMaximum
			}
			tryCount++
			continue
		}

		return err
	}

	return nil
}

const (
	MaxSetDataTryCount = 10
)

// SetDataIntoMap znode의 value 가 json 형태의 데이터라고 가정하고 해당 json 데이터에서 특정 키 path 에 특정 값을 put 한다
func (z *ZkServant) SetDataIntoMap(znodePath, keyPath string, data interface{}) error {
	zk.DefaultLogger.Printf("zk SetDataIntoMap [%s]", znodePath)

	handlerFunc := func(previousData []byte) ([]byte, error) {
		var previous M
		err := json.Unmarshal(previousData, &previous)
		if err != nil {
			return nil, fmt.Errorf("SetDataIntoMap.Unmarshal error [%s] : %s", znodePath, err.Error())
		}
		err = previous.SetKey(keyPath, data)
		if err != nil {
			return nil, fmt.Errorf("SetDataIntoMap.SetKey error [%s] : %s", znodePath, err.Error())
		}
		return json.Marshal(previous)
	}

	return z.SetDataWithVersion(znodePath, handlerFunc)
}

const (
	FlagPersistent      = 0
	TransferH2          = "h2"
	TransferH2ClearText = "h2c"
)

func (z *ZkServant) ensurePath(path string) error {
	zk.DefaultLogger.Printf("zk ensurePath [%s]", path)

	acl := zk.WorldACL(zk.PermAll)
	dir := ""
	for _, p := range strings.Split(path, "/") {
		if len(p) == 0 {
			continue
		}
		dir += "/" + p

		_, err := z.zkConn.Create(dir, nil, FlagPersistent, acl)
		if err != nil && err != zk.ErrNodeExists {
			return err
		}
		zk.DefaultLogger.Printf("[%s] created", dir)
	}

	return nil
}

func (z *ZkServant) GetTransportMode(path string) (TransportMode, error) {
	zk.DefaultLogger.Printf("GetTransportMode : %s", path)
	b, err := z.GetData(path)
	if err != nil {
		return TransportModeUnknown, err
	}

	var data map[string]string
	err = json.Unmarshal(b, &data)
	if err != nil {
		return TransportModeUnknown, err
	}

	// e.g {"transport":"h2c"}
	mode, ok := data[ServiceKeyTransport]
	if !ok {
		return TransportModeUnknown, fmt.Errorf("Invalid service description. not found transport key")
	}

	switch strings.ToLower(mode) {
	case TransferH2ClearText:
		return TransportModePlain, nil
	case TransferH2:
		return TransportModeSsl, nil
	}

	return TransportModeUnknown, nil
}
func (z *ZkServant) GetData(path string) ([]byte, error) {
	zk.DefaultLogger.Printf("zk GetData : %s", path)
	z.ensureZkEnabled()
	data, _, err := z.zkConn.Get(path)
	return data, err
}

func (z *ZkServant) GetDataWithStat(path string) ([]byte, *zk.Stat, error) {
	zk.DefaultLogger.Printf("zk GetDataWithStat : %s", path)
	z.ensureZkEnabled()
	data, stat, err := z.zkConn.Get(path)
	return data, stat, err
}

func (z *ZkServant) ChildrenW(path string) ([]string, <-chan zk.Event, error) {
	z.ensureZkEnabled()
	data, _, ch, err := z.zkConn.ChildrenW(path)
	return data, ch, err
}

func (z *ZkServant) Delete(path string) error {
	zk.DefaultLogger.Printf("zk deleteNode [%s]", path)
	return z.zkConn.Delete(path, zkVersionAll)
}

func (z *ZkServant) createLockPath(path string) error {
	acl := zk.WorldACL(zk.PermAll)
	_, err := z.zkConn.Create(path, nil, zk.FlagEphemeral, acl)
	if err != nil {
		return err
	}

	return nil
}

func (z *ZkServant) deleteLockPath(path string) {
	err := z.zkConn.Delete(path, 0)
	if err != nil {
		zk.DefaultLogger.Printf("Delete error : %s, %s", path, err.Error())
		return
	}
}

func parseIpList(config string) []string {
	items := strings.Split(config, ",")

	list := make([]string, 0)
	for _, v := range items {
		list = append(list, strings.Trim(v, " "))
	}

	return list
}

const (
	pathDelimiter = "."
)

type M map[string]interface{}

func (m M) SetKey(keyPath string, value interface{}) error {
	if len(keyPath) == 0 {
		return fmt.Errorf("empty keypath")
	}

	if strings.Contains(keyPath, " ") ||
		strings.HasPrefix(keyPath, pathDelimiter) ||
		strings.HasSuffix(keyPath, pathDelimiter) {
		return fmt.Errorf("invalid keypath [%s]", keyPath)
	}

	pathTokenList := strings.Split(keyPath, pathDelimiter)

	if len(pathTokenList) == 1 {
		key := pathTokenList[0]
		if len(key) == 0 {
			return fmt.Errorf("empty key for keyPath [%s]", keyPath)
		}
		m[key] = value
		return nil
	}

	// A.B.C 에 value를 세팅한다는것은 A.B 에 C라는 키로 value를 세팅하는것이므로 A.B까지만 찾는다
	var err error
	var pathFound = false
	node := m
	for i := 0; i < len(pathTokenList)-1; i++ {
		key := pathTokenList[i]
		if len(key) == 0 {
			continue
		}

		node, err = node.getKey(key, true)
		if err != nil {
			return fmt.Errorf("getKey[%s] error : %s", pathTokenList[i], err.Error())
		}
		pathFound = true
	}

	if !pathFound {
		return fmt.Errorf("abnormal keyPath [%s]", keyPath)
	}

	leaf := pathTokenList[len(pathTokenList)-1]
	node[leaf] = value
	return nil
}

func (m M) GetKey(key string) (M, error) {
	return m.getKey(key, false)
}

func (m M) getKey(key string, create bool) (M, error) {
	value, ok := m[key]
	if !ok {
		if !create {
			return nil, fmt.Errorf("not found key [%s]", key)
		}

		// key가 존재하지 않으면 새로 생성한다
		mappedValue := make(M)
		m[key] = mappedValue
		return mappedValue, nil
	}

	mappedValue, ok := value.(map[string]interface{})
	if ok {
		return mappedValue, nil
	}

	xType := reflect.TypeOf(value)
	xValue := reflect.ValueOf(value)
	return nil, fmt.Errorf("%s 의 하위 데이터 구조가 map 형식이 아닙니다. type=[%s], value=[%s]", key, xType, xValue)
}
