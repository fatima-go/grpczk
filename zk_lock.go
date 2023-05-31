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
 * @date 20. 3. 10. 오후 5:51
 */

package grpczk

import (
	"github.com/go-zookeeper/zk"
	"path/filepath"
	"time"
)

type ZkLock struct {
	err       error
	zkServant *ZkServant
	lockPath  string
}

func (z ZkLock) Error() error {
	return z.err
}

func (z *ZkLock) Close() {
	if z.zkServant != nil && len(z.lockPath) > 0 {
		z.zkServant.deleteLockPath(z.lockPath)
	}
}

func (z *ZkServant) AcquireLock(znodeBaseDir, key string, opts ...*LockOptions) ZkLock {
	znodePath := filepath.Join(znodeBaseDir, key)

	lockOpts := mergeLockOptions(opts...)
	zkLock := ZkLock{}
	zkLock.lockPath = znodePath
	zkLock.zkServant = z
	first := true

	for {
		err := z.createLockPath(znodePath)
		if err == nil {
			if !first {
				zk.DefaultLogger.Printf("acquire lock for key [%s]", key)
			}
			return zkLock
		}

		if err != zk.ErrNodeExists {
			zkLock.err = err
			return zkLock
		}

		// Node Exist...
		if lockOpts.Giveup != nil && *lockOpts.Giveup {
			zkLock.err = ErrLockGiveup
			return zkLock
		}

		if first {
			first = false
		}

		children, eventCh, err := z.ChildrenW(znodeBaseDir)
		if err != nil {
			zkLock.err = err
			return zkLock
		}

		// check list has a key
		if !hasKeyInList(children, key) {
			// 발견되지 않았다면 다시 생성을 시도한다
			continue
		}

		zk.DefaultLogger.Printf("waiting key [%s] release....", key)

		for {
			exit := false
			select {
			case e := <-eventCh:
				children, eventCh, err = z.ChildrenW(znodeBaseDir)
				if err != nil {
					zkLock.err = err
					return zkLock
				}

				if e.Type&zk.EventNodeChildrenChanged != zk.EventNodeChildrenChanged {
					break
				}

				// check list has a key
				if !hasKeyInList(children, key) {
					// 발견되지 않았다면 다시 생성을 시도한다
					exit = true
				}
			case <-time.After(time.Second * 5):
				exit = true
				zk.DefaultLogger.Printf("[%s] timeout...", key)
			}

			if exit {
				break
			}
		}
	}
}

func hasKeyInList(list []string, key string) bool {
	if len(list) == 0 {
		return false
	}

	for _, v := range list {
		if v == key {
			return true
		}
	}
	return false
}
