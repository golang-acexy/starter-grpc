# starter-grpc

基于`google.golang.org/grpc`封装的gRPC服务端/客户端组件

---

#### 功能说明

支持快速开启服务端/客户端，支持Client模式使用resolver服务发现模式

- resolver 模式

  - etcd 动态服务发现

    ```go
    // 使用动态服务器路由列表(基于etcd) 启动 grpcstarter_test.go -> TestStartMoreSrv 启动一批服务端
    func TestCallServerWithEtcdResolver(t *testing.T) {
    etcdSrv := "http://localhost:2379"
    
        etcdResolver := &resolver.Etcd{EtcdUrls: []string{etcdSrv}}
        conn, err := grpcstarter.NewClientConnWithResolver(resolver.EtcdScheme+":///users", etcdResolver,
            grpc.WithTransportCredentials(insecure.NewCredentials()),               // 免认证
            grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // 使用负载策略 (如果不使用负载策略则不会在服务器列表中使用负载功能，可能一直请求同一个服务器)
        )
    
        if err != nil {
            fmt.Printf("%v\n", err)
              return
        }
    
        // 开启一个异步协程，5秒后，将相关服务端实例注册到etcd，测试本客户端是否可以感知并开始请求
        go func() {
            // 也可以通过直接操作etcd将相关服务端进行注册
            // etcdctl get "" --prefix=true 查看所有key
            // 手动注册服务端实例至etcd
            // etcdctl put "users/1" '{"Addr":"localhost:8085"}'
            // etcdctl put "users/2" '{"Addr":"localhost:8084"}'
            // etcdctl put "users/3" '{"Addr":"localhost:8083"}'
            ctx, cancel := context.WithCancel(context.Background())
            time.Sleep(time.Second * 5)
            fmt.Println("register new instance")
            etcdResolver.RegisterEtcdSrvInstance(ctx, "users", "1", "localhost:8085", 3)
            etcdResolver.RegisterEtcdSrvInstance(ctx, "users", "2", "localhost:8084", 3)
            etcdResolver.RegisterEtcdSrvInstance(ctx, "users", "3", "localhost:8083", 3)
            etcdResolver.RegisterEtcdSrvInstance(ctx, "users", "4", "localhost:8082", 10)
            time.Sleep(time.Second * 40)
            fmt.Println("stop instance keepalive")
            cancel()
        }()
    
        ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
        defer cancel()
        doRequest(ctx, conn)
        <-ctx.Done()
    }
    ```    

  - static 静态服务列表配置

    ````go
    // 使用静态服务端列表 启动 grpcstarter_test.go -> TestStartMoreSrv 启动一批服务端
    func TestCallServerWithStaticResolver(t *testing.T) {
      conn, err := grpcstarter.NewClientConnWithResolver(resolver.StaticScheme+":///users", resolver.Static{Addresses: map[string][]string{
        "users": {
          "127.0.0.1:8085",
          "127.0.0.1:8084",
          "127.0.0.1:8083",
          "127.0.0.1:8082",
          "127.0.0.1:8081",
        },
      }},
        grpc.WithTransportCredentials(insecure.NewCredentials()),               // 免认证
        grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // 使用负载策略 (如果不使用负载策略则不会在服务器列表中使用负载功能，可能一直请求同一个服务器)
      )
      if err != nil {
        fmt.Printf("%v\n", err)
      }

      ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
      defer cancel()
      doRequest(ctx, conn)
      <-ctx.Done()
    }
    ````
