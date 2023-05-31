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
 * @date 21. 7. 6. 오후 5:32
 */

package grpczk

import "errors"

var (
	ErrLockGiveup = errors.New("zklock : giveup locking")
)

type LockOptions struct {
	// If true, Caller receive Lock(with giveup field as true) while fail to acquire lock
	// It means if acquiring fails at first time, caller doesn't try acquiring.
	Giveup *bool
}

func (l *LockOptions) SetGiveup(b bool) *LockOptions {
	l.Giveup = &b
	return l
}

// MergeFindOptions combines the given LockOptions instances into a single LockOptions in a last-one-wins fashion.
func mergeLockOptions(opts ...*LockOptions) *LockOptions {
	lo := &LockOptions{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if opt.Giveup != nil {
			lo.Giveup = opt.Giveup
		}
	}

	return lo
}
