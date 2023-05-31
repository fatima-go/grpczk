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
 * @date 21. 5. 12. 오후 3:13
 */

package grpczk

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMapHandling(t *testing.T) {
	var root M
	err := json.Unmarshal([]byte(sampleJsonText), &root)
	if err != nil {
		t.Fatalf("invalid root error : %s", err.Error())
		return
	}

	node := CallGetKey(t, root, "global", false)
	node = CallGetKey(t, node, "vcoloring", false)

	_, err = node.GetKey("not_exist_node")
	if err == nil {
		t.Fatalf("GetKey [%s] : creation fail", "not_exist_node")
	}

	CallGetKey(t, node, "unknown_node", true)
	node = CallGetKey(t, node, "link_popup", false)

	err = root.SetKey("myroot", "test")
	if err != nil {
		t.Fatalf("SetKey [%s] : %s", "myroot", err.Error())
	}

	err = root.SetKey("global.vcoloring.link_popup.button_link_text", "mybuttonlink")
	if err != nil {
		t.Fatalf("SetKey [%s] : %s", "global.vcoloring.link_popup.button_link_text", err.Error())
	}

	kv := make(map[string]interface{})
	kv["test1"] = "test1"
	kv["test2"] = 123456789

	err = root.SetKey("global.vcoloring.link_popup.top_link_text", kv)
	if err != nil {
		t.Fatalf("SetKey [%s] : %s", "global.vcoloring.link_popup.top_link_text", err.Error())
	}

	// debug...
	body, _ := json.MarshalIndent(root, "", "    ")
	bodyText := string(body)
	assertExistString(t, bodyText, "unknown_node")
	assertExistString(t, bodyText, "myroot")
	assertExistString(t, bodyText, "mybuttonlink")
	assertExistString(t, bodyText, "test1")
	log.Printf("\n%s\n", body)
}

func assertExistString(t *testing.T, text string, key ...string) {
	for _, v := range key {
		if !strings.Contains(text, v) {
			t.Fatalf("not found value : [%s]", v)
		}
	}
}

func CallGetKey(t *testing.T, node M, key string, create bool) M {
	var err error

	if create {
		node, err = node.getKey(key, create)
		if err != nil {
			t.Fatalf("GetKey [%s] : %s", key, err.Error())
		}
	} else {
		node, err = node.GetKey(key)
		if err != nil {
			t.Fatalf("GetKey [%s] : %s", key, err.Error())
		}
	}

	return node
}

var sampleJsonText = `{
    "global" : {
        "vcoloring": {
            "link_popup": {
                "introduction_text": "이 뮤직비디오를 내 통화화면으로 설정할 수 있어요.",
                "top_link_text": "V컬러링 소개보기",
                "top_link_url": "https://tworld.com",
                "button_link_text": "설정하러 가기",
                "button_link_url": "https://link.vcoloring.com/?link=https%3A%2F%2Fvcoloring.com%2Fvideo%2Fflo${vcId}"
            }
        }
    },
    "domain" : {
        "display-api" : {
            "disabled_api_list" : ["/purchase/v1/promotion/targets"],
            "popup_update_dtime" : "2021-04-15 19:35:12"
        }
    }
}`

const (
	znodeSetTestWithVersion = "/dave_set_test"
)

func TestSetDataIntoMap(t *testing.T) {
	zkServant := NewZkServant(zkIpList)
	err := zkServant.Connect()
	if err != nil {
		log.Printf("fail to connect zk : %s", err.Error())
		return
	}

	// prepare node
	zkServant.Delete(znodeSetTestWithVersion)
	err = zkServant.SetData(znodeSetTestWithVersion, []byte(sampleJsonText))
	if err != nil {
		t.Fatalf("SetData err : %s", err.Error())
	}

	keyPath := "global.vcoloring.test_update_dtime"
	updateDtime := time.Now().Format("2006-01-02 15:04:05")
	err = zkServant.SetDataIntoMap(znodeSetTestWithVersion, keyPath, updateDtime)
	if err != nil {
		t.Fatalf("SetDataIntoMap err : %s", err.Error())
	}

	data, err := zkServant.GetData(znodeSetTestWithVersion)
	if err != nil {
		t.Fatalf("GetData err : %s", err.Error())
	}
	if !strings.Contains(string(data), "test_update_dtime") {
		t.Fatalf("invalid expected data. not found test_update_dtime")
	}
}

func TestSetDataWithVersion(t *testing.T) {
	zkServant := NewZkServant(zkIpList)
	err := zkServant.Connect()
	if err != nil {
		log.Printf("fail to connect zk : %s", err.Error())
		return
	}

	// prepare node
	zkServant.Delete(znodeSetTestWithVersion)
	initialCount := "1"
	err = zkServant.SetData(znodeSetTestWithVersion, []byte(initialCount))
	if err != nil {
		t.Fatalf("SetData err : %s", err.Error())
	}

	// start racing
	exceedMaxRetryErrCount = 0
	loopCount := 10
	threadCount := 10

	wg := sync.WaitGroup{}
	wg.Add(threadCount)

	for i := 0; i < threadCount; i++ {
		go startZkSetTest(i, &wg, loopCount)
	}

	wg.Wait()

	// check
	data, err := zkServant.GetData(znodeSetTestWithVersion)
	if err != nil {
		t.Fatalf("GetData err : %s", err.Error())
	}
	finalCount, _ := strconv.Atoi(string(data))
	if finalCount != 101-int(exceedMaxRetryErrCount) {
		t.Fatalf("invalid final count : %d", finalCount)
	}
}

var exceedMaxRetryErrCount int32

func startZkSetTest(workerId int, wg *sync.WaitGroup, loop int) {
	defer wg.Done()

	zkServant := NewZkServant(zkIpList)
	err := zkServant.Connect()
	if err != nil {
		log.Printf("fail to connect zk : %s", err.Error())
		return
	}

	handlerFunc := func(previousData []byte) ([]byte, error) {
		count, err := strconv.Atoi(string(previousData))
		if err != nil {
			return nil, err
		}

		count++
		return []byte(strconv.Itoa(count)), nil
	}

	for i := 0; i < loop; i++ {
		err = zkServant.SetDataWithVersion(znodeSetTestWithVersion, handlerFunc)
		if err != nil {
			log.Printf("[%d] zk SetDataWithVersion error : %s", workerId, err.Error())
			if strings.Contains(err.Error(), "exceed max retry") {
				atomic.AddInt32(&exceedMaxRetryErrCount, 1)
			}
		}
	}
}
