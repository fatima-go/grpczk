# grpczk #
apache zookeeper 를 통해서 advertise 된 grpc 서버의 정보를 실시간으로 감시하여 grpc client에서 grpc server list 를 갱신하도록 기능을 제공한다

# release #

2025.04.19 v1.2.1
- LICENSE.md 파일 추가

2025.04.08 v1.2.0
- [ZkServant.Close() 시에 hostProvider ticker 종료](https://github.com/fatima-go/grpczk/issues/21)

2025.04.03 v1.1.0
- [Zookeeper 서버의 IP 가 변경되었을때 대응 로직 추가](https://github.com/fatima-go/grpczk/issues/19)

2025.03.10 v1.0.9
- [명시적 Close() 처리시 별도 이벤트 처리 go func에 의한 오동작 수정](https://github.com/fatima-go/grpczk/issues/17)

2025.02.27 v1.0.8
- [명시적 Close() 호출 이후에 nil 포인트 에러 대응](https://github.com/fatima-go/grpczk/issues/15)

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