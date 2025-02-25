# grpczk #
apache zookeeper 를 통해서 advertise 된 grpc 서버의 정보를 실시간으로 감시하여 grpc client에서 grpc server list 를 갱신하도록 기능을 제공한다

# release #

2025.02.25 v1.0.7
- [disconnect 되었을때 연결 재설정 코드 필요](https://github.com/fatima-go/grpczk/issues/13)

2024.07.26 v1.0.6
- [zk_client 에서 nil 에러](https://github.com/fatima-go/grpczk/issues/11)

2024.1.15 v1.0.5
- [zk 노드를 통해 서버 목록 처리시 클라이언트에 확장성 제공 ](https://github.com/fatima-go/grpczk/issues/9)

2024.1.11 v1.0.4
- [znode 의 서비스에 별도 로드밸런서를 제공하는 인터페이스 추가](https://github.com/fatima-go/grpczk/issues/6)

2023.11.29 v1.0.3
- [zkServant.Close() 시에 nil 포인트 에러](https://github.com/fatima-go/grpczk/issues/3)

2023.11.09 v1.0.2
- [updateServerList()의 nil 포인트 에러](https://github.com/fatima-go/grpczk/issues/1) 해결
- github.com/go-zookeeper/zk 버전 1.0.3 업데이트
- google.golang.org/protobuf 버전 v1.31.0 업데이트
- google.golang.org/grpc 버전 v1.59.0 업데이트