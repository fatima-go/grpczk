# grpczk #
apache zookeeper 를 통해서 advertise 된 grpc 서버의 정보를 실시간으로 감시하여 grpc client에서 grpc server list 를 갱신하도록 기능을 제공한다

# release #
2023.11.09 v1.0.4
- [updateServerList()의 nil 포인트 에러](https://github.com/fatima-go/grpczk/issues/1) 해결
- github.com/go-zookeeper/zk 버전 1.0.3 업데이트
- google.golang.org/protobuf 버전 v1.31.0 업데이트
- google.golang.org/grpc 버전 v1.59.0 업데이트