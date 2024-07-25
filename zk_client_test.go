package grpczk

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	hostPorts = "localhost:2181,localhost:2182,localhost:2183"
)

var (
	zkConn *testZkConn
)

func TestNewZkClientServant_Connect(t *testing.T) {
	err := createTestZkConn()
	assert.NoError(t, err, "")
	fixtures := make([]fixture, 0)
	for i := 0; i < 3; i++ {
		f := fixture{
			path: fmt.Sprintf("/test/node%d", i),
			port: 8888 + i,
		}
		assert.NoError(t, startServer(f), "")
		assert.NoError(t, err, "")
		fixtures = append(fixtures, f)
	}

	prepare := func() {
		zkClientServant = nil
		clientServant := NewZkClientServant(hostPorts)

		for _, f := range fixtures {
			err = zkConn.deleteAllChildren(f.path)
			if err == nil {
				err = zkConn.createPath(fmt.Sprintf("%s/127.0.0.1:%d", f.path, f.port), zk.FlagEphemeral)
			}

			if err == nil {
				_, err = clientServant.Connect(f.path)
			}

			if err != nil {
				assert.Fail(t, err.Error())
				return
			}
		}
	}

	t.Parallel()

	t.Run("Create 확인", func(subT *testing.T) {
		clientServant := NewZkClientServant(hostPorts)

		assert.NotNil(t, clientServant.zkServant)
		assert.Nil(t, clientServant.errorLogger)
	})

	t.Run("PANIC 미발생 확인", func(subT *testing.T) {
		notify := func(f fixture, sleep int, ctx context.Context) {
			for serverIndex := 1; serverIndex <= 5; serverIndex++ {
				i := serverIndex
				go func() {
					for loop := 0; loop < 100; loop++ {
						select {
						case <-ctx.Done():
							return
						default:
						}

						path := f.path + "/" + fmt.Sprintf("127.0.0.%d:%d", i, f.port)
						_ = zkConn.createPath(path, zk.FlagEphemeral)
						time.Sleep(time.Duration(sleep+2) * time.Millisecond)
					}
				}()
			}
		}

		for i := 0; i < 50; i++ {
			prepare()

			wg := sync.WaitGroup{}
			wg.Add(len(fixtures))
			ctx, ctxCancel := context.WithCancel(context.Background())

			for index, f := range fixtures {
				_f := f
				_i := index
				go notify(_f, _i, ctx)
				go func() {
					defer wg.Done()
					time.Sleep(time.Duration(_i+20) * time.Millisecond)
					zkClientServant.Disconnect(_f.path)
				}()
			}

			wg.Wait()
			ctxCancel()
		}
	})

	t.Run("정상 종료 확인", func(subT *testing.T) {
		notify := func(f fixture) {
			wg := sync.WaitGroup{}
			wg.Add(3)

			for serverIndex := 1; serverIndex <= 3; serverIndex++ {
				i := serverIndex
				go func() {
					defer wg.Done()

					for loop := 0; loop < 20; loop++ {
						path := f.path + "/" + fmt.Sprintf("127.0.0.%d:%d", i, f.port)
						_ = zkConn.createPath(path, zk.FlagEphemeral)
						time.Sleep(3 * time.Millisecond)
					}
				}()
			}

			wg.Wait()
		}

		for i := 0; i < 5; i++ {
			prepare()

			wg := sync.WaitGroup{}
			wg.Add(len(fixtures))

			for _, f := range fixtures {
				_f := f
				go func() {
					defer wg.Done()
					notify(_f)
				}()
			}

			wg.Wait()
			for _, f := range fixtures {
				zkClientServant.Disconnect(f.path)
			}

			assert.Nil(subT, zkClientServant)
			time.Sleep(500 * time.Millisecond)
		}
	})
}

func TestNewZkClientServant_UpdateServer(t *testing.T) {
	err := createTestZkConn()
	assert.NoError(t, err, "")
	fixtures := make([]fixture, 0)
	for i := 0; i < 2; i++ {
		f := fixture{
			path:   fmt.Sprintf("/test/node%d", i),
			port:   9000 + i,
			helper: &zkServiceHelper{},
		}

		assert.NoError(t, startServer(f), "")
		err = zkConn.createPath(fmt.Sprintf("%s/127.0.0.1:%d", f.path, f.port), zk.FlagEphemeral)
		fixtures = append(fixtures, f)
	}

	clientServant := NewZkClientServant(hostPorts)

	serverMap := make(map[string][]string)
	for _, f := range fixtures {
		_, err := clientServant.ConnectWithHelper(f.path, f.helper)
		assert.NoError(t, err)

		servers := make([]string, 0)
		for i := 0; i < 50; i++ {
			server := fmt.Sprintf("127.0.0.%d:%d", i, f.port)
			path := fmt.Sprintf("%s/%s", f.path, server)
			servers = append(servers, server)
			err = zkConn.createPath(path, zk.FlagEphemeral)
			assert.NoError(t, err)
		}
		serverMap[f.path] = servers
	}

	time.Sleep(3 * time.Second)

	for _, f := range fixtures {
		assert.Equal(t, len(serverMap[f.path]), len(f.helper.foundAddrList))
		for _, expectedServer := range serverMap[f.path] {
			found := false
			for _, actualServer := range f.helper.foundAddrList {
				if expectedServer == actualServer {
					found = true
					break
				}
			}
			if !found {
				assert.Fail(t, "서버 목록 불일치")
			}
		}
	}
}

type testZkConn struct {
	conn *zk.Conn
}

func createTestZkConn() error {
	if zkConn != nil {
		return nil
	}

	servers := strings.Split(hostPorts, ",")
	conn, _, err := zk.Connect(servers, 10*time.Second)

	if err != nil {
		return err
	}

	zkConn = &testZkConn{conn: conn}
	return nil
}

func (t *testZkConn) deleteAllChildren(path string) error {
	children, _, _ := t.conn.Children(path)
	if len(children) == 0 {
		return nil
	}

	for _, child := range children {
		err := t.conn.Delete(path+"/"+child, -1)
		if err != nil {
			if errors.Is(err, zk.ErrNoNode) {
				continue
			}
			return err
		}
	}

	return nil
}

func (t *testZkConn) deletePath(path string) error {
	return t.conn.Delete(path, -1)
}

func (t *testZkConn) createPath(path string, flag int32) error {
	p, sep := "", "/"
	for _, s := range strings.Split(path, sep) {
		if len(s) == 0 {
			continue
		}
		p = p + sep + s
		_, e := t.conn.Create(p, nil, flag, zk.WorldACL(zk.PermAll))
		if e != nil {
			if errors.Is(e, zk.ErrNodeExists) {
				continue
			}
			return e
		}
	}
	return nil
}

func (t *testZkConn) setData(path, data string) error {
	_, err := t.conn.Set(path, []byte(data), -1)
	return err
}

func (t *testZkConn) close() {
	if t.conn != nil {
		t.conn.Close()
	}
}

type zkServiceHelper struct {
	foundAddrList []string
}

func (z *zkServiceHelper) UpdateServerList(foundAddrList []string) []string {
	z.foundAddrList = foundAddrList
	return z.foundAddrList
}

func (z *zkServiceHelper) GetBalancerName() string {
	return ""
}

type fixture struct {
	path   string
	port   int
	helper *zkServiceHelper
}

func (f fixture) portAsStr() string {
	return strconv.Itoa(f.port)
}

func startServer(f fixture) error {
	err := zkConn.createPath(f.path, 0)
	if err == nil {
		err = zkConn.setData(f.path, "{\"transport\":\"h2c\"}")
	}

	var grpcServerErrorChan chan error
	go func() {
		listener, e := net.Listen("tcp", ":"+f.portAsStr())
		if e == nil {
			e = grpc.NewServer().Serve(listener)
		}
		grpcServerErrorChan <- e
	}()

	select {
	case err = <-grpcServerErrorChan:
		return err
	case <-time.After(time.Second):
	}

	return nil
}
