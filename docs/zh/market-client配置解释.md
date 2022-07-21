# Market Client 配置解释

market-client 的一份典型的配置如下

```

SimultaneousTransfersForRetrieval = 20
SimultaneousTransfersForStorage = 20
DefaultMarketAddress = "t3qkgm5h7nmusacfggd744w7fdj45rn6iyl7n6s6lr34t4qlfebiphmm3vxtwc4a4acqi4nv3pqk6h7ddqqz5q"

[API]
  ListenAddress = "/ip4/127.0.0.1/tcp/41231/ws"
  RemoteListenAddress = ""
  Secret = ""
  Timeout = "30s"

[Libp2p]
  ListenAddresses = ["/ip4/0.0.0.0/tcp/34123", "/ip6/::/tcp/0"]
  AnnounceAddresses = []
  NoAnnounceAddresses = []
  PrivateKey = ""

[Node]
  Url = "/ip4/192.168.200.106/tcp/3453"
  Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiYWRtaW4iLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.0fylyMSNjp8dkTrCLYkFQSjO9FokDKXrl5dqGpMDaOE"

[Messager]
  Url = ""
  Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiYWRtaW4iLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.0fylyMSNjp8dkTrCLYkFQSjO9FokDKXrl5dqGpMDaOE"

[Signer]
  Type = ""
  Url = ""
  Token = ""

```

其中，可以分成三个部分： client 网络配置，venus 服务组件的配置和 其他配置

## Market Client 网络配置

这部分的配置决定了venus client 和外界交互的接口

### [API]
market-client 对外提供服务的接口

```
[API]
# market-client 提供服务监听的地址
# 字符串类型，必选项，默认为:"/ip4/127.0.0.1/tcp/41235"
ListenAddress = "/ip4/127.0.0.1/tcp/41235"

# 保留字段
RemoteListenAddress = ""

# 密钥用于加密通信
#字符串类型 可选项（没有则自动生成）
Secret = "878f9c1f88c6f68ee7be17e5f0848c9312897b5d22ff7d89ca386ed0a583da3c"

#保留字段
Timeout = "30s"
```

### [Libp2p]

在P2P网络中通信时使用的 通信地址
```
[Libp2p]
# 监听的网络地址
# 字符串数组 必选 默认为:["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]
ListenAddresses = ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]

# 保留字段
AnnounceAddresses = []

# 保留字段
NoAnnounceAddresses = []

# 用于p2p加密通信的私钥
# 字符串 可选（没设置则自动生成）
PrivateKey = "08011240ae580daabbe087007d2b4db4e880af10d582215d2272669a94c49c854f36f99c35"
```



## venus 组件服务配置

当market-client接入venus组件使用时，需要配置相关组件的API。

### [Node]
venus链服务接入配置
```
[Node]
# 链服务的入口
# 字符串类型 必选（也可以直接通过命令行的--node-url flag 进行配置）
Url = "/ip4/192.168.200.128/tcp/3453"

# venus 系列组件的鉴权token
# 字符串类型 必选（也可以直接通过命令行的 --auth-token flag 进行配置）
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"

```


### [Messager]

venus 消息服务接入配置

```
[Messager]
# 消息服务入口
# 字符串类型 必选（也可以直接通过命令行的 --messager-url flag 进行配置）
Url = "/ip4/192.168.200.128/tcp/39812/"

# venus 系列组件的鉴权token
# 字符串类型 必选（也可以直接通过命令行的 --auth-token flag 进行配置）
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


### [Signer]

venus 提供签名服务的组件
在 market-client 只能使用 wallet 类型的签名服务

```
[Signer]
# 签名服务组件的类型
# 字符串类型  只能是"wallet"
Type = "wallet"

# 签名服务入口
# 字符串类型 必选（也可以直接通过命令行的 --signer-url flag 进行配置）
Url = "/ip4/192.168.200.128/tcp/5678/"

# venus 系列组件的鉴权token
# 字符串类型 必选（也可以直接通过命令行的 --auth-token flag 进行配置）
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


## 其他配置

```
# 获取数据订单进行同时传输的最大数量
# 整数类型 默认：20
SimultaneousTransfersForRetrieval = 20

# 存储订单同时进行传输的最大数量
# 整数类型 默认：20
SimultaneousTransfersForStorage = 20

# 当前market-client的默认地址
# 字符串类型 可选 （也可以直接通过命令行的 --addr flag 进行配置）
DefaultMarketAddress = "t3qkgm5h7nmusacfggd744w7fdj45rn6iyl7n6s6lr34t4qlfebiphmm3vxtwc4a4acqi4nv3pqk6h7ddqqz5q"
```
