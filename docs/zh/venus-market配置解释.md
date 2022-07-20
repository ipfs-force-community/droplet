# venus market 的配置解释

一份典型的venus market 的配置是这样的:
```

# ****** 基础参数配置 ********
ConsiderOnlineStorageDeals = true
ConsiderOfflineStorageDeals = true
ConsiderOnlineRetrievalDeals = true
ConsiderOfflineRetrievalDeals = true
ConsiderVerifiedStorageDeals = true
ConsiderUnverifiedStorageDeals = true
PieceCidBlocklist = []
ExpectedSealDuration = "24h0m0s"
MaxDealStartDelay = "336h0m0s"
PublishMsgPeriod = "5m0s"
MaxDealsPerPublishMsg = 8
MaxProviderCollateralMultiplier = 2
SimultaneousTransfersForStorage = 20
SimultaneousTransfersForStoragePerClient = 20
SimultaneousTransfersForRetrieval = 20
Filter = ""
RetrievalFilter = ""
TransfePath = ""
MaxPublishDealsFee = "0 FIL"
MaxMarketBalanceAddFee = "0 FIL"


# ****** venus market 网络配置  ********

[API]
ListenAddress = "/ip4/0.0.0.0/tcp/41235"
RemoteListenAddress = ""
Secret = "878f9c1f88c6f68ee7be17e5f0848c9312897b5d22ff7d89ca386ed0a583da3c"
Timeout = "30s"

[Libp2p]
ListenAddresses = ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]
AnnounceAddresses = []
NoAnnounceAddresses = []
PrivateKey = "08011240ae580daabbe087007d2b4db4e880af10d582215d2272669a94c49c854f36f99c35c38130ac8731dedae9cc885c644554d3e4ca9203ffeeeb9ee7a689a3e52a21"


# ****** venus 组件服务配置 ********
[Node]
Url = "/ip4/192.168.200.128/tcp/3453"
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"

[Messager]
Url = "/ip4/192.168.200.128/tcp/39812/"
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"

[Signer]
Type = "gateway"
Url = "/ip4/192.168.200.128/tcp/45132/"
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"

[AuthNode]
Url = "http://192.168.200.128:8989"
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"



#  ******** 数据库设置 ********
[Mysql]
ConnectionString = ""
MaxOpenConn = 100
MaxIdleConn = 100
ConnMaxLifeTime = "1m"
Debug = false


# ******** 扇区存储设置 ********
[PieceStorage]
S3 = []

[[PieceStorage.Fs]]
Name = "local"
ReadOnly = false
Path = "./.vscode/test"


# ******** 日志设置 ********
[Journal]
Path = "journal"


# ******** 消息发送地址的配置 ********
[AddressConfig]
DisableWorkerFallback = false


# ******** DAG存储设置 ********

[DAGStore]
RootDir = "/root/.venusmarket/dagstore"
MaxConcurrentIndex = 5
MaxConcurrentReadyFetches = 0
MaxConcurrencyStorageCalls = 100
GCInterval = "1m0s"
Transient = ""
Index = ""
UseTransient = false


# ******** 数据索取配置 ********

[RetrievalPaymentAddress]
Addr = ""
Account = ""

[RetrievalPricing]
Strategy = "default"
[RetrievalPricing.Default]
VerifiedDealsFreeTransfer = true
[RetrievalPricing.External]
Path = ""

```

接下来，将这个配置分成基础参数，网络配置，Venus组件配置等多个部分进行讲解

## 基础参数配置

这部分的配置主要是决定了了market在进行工作时的偏好，满足定制化的需求，其中各项配置的作用如下：

``` 
# 决定是否接受线上存储订单
# 布尔值 默认为 true
ConsiderOnlineStorageDeals = true

# 决定是否接受线下存储订单
# 布尔值 默认为 true
ConsiderOfflineStorageDeals = true

# 决定是否接受线上数据获取订单
# 布尔值 默认为 true
ConsiderOnlineRetrievalDeals = true

# 决定是否接受线下数据获取订单
# 布尔值 默认为 true
ConsiderOfflineRetrievalDeals = true

# 决定是否接受经过验证的存储订单
# 布尔值 默认为 true
ConsiderVerifiedStorageDeals = true

# 决定是否接受未经过验证的存储订单
# 布尔值 默认为 true
ConsiderUnverifiedStorageDeals = true

# 订单数据黑名单
# 字符串数组 其中每一个字符串都是CID 默认为空
# CID在黑名单中的数据，不会被用于构建订单
PieceCidBlocklist = []

# 订单数据被封装完成的最大时间预期
# 时间字符串 默认为："24h0m0s"
# 时间字符串是由数字和时间单位组成的字符串，数字包括整数和小数，合法的单位包括 "ns", "us" (or "µs"), "ms", "s", "m", "h".
ExpectedSealDuration = "24h0m0s"

# 预期订单开始epoch出现的最大延迟
# 时间字符串 默认为："336h0m0s"
MaxDealStartDelay = "336h0m0s"

# 消息推送上链的周期
# 时间字符串 默认为："1h0m0s"
PublishMsgPeriod = "5m0s"

# 在一个消息推送周期内的最大订单书数量
# 整数类型 默认为8 
MaxDealsPerPublishMsg = 8

# 最大的存储供应商抵押乘法因子
# 整数类型 默认为：2
MaxProviderCollateralMultiplier = 2

# 存储订单的最大同时传输数目
# 整数类型 默认为：20
SimultaneousTransfersForStorage = 20

# 针对每一个客户端的存储订单最大同时传输数目
# 整数类型 默认为：20
SimultaneousTransfersForStoragePerClient = 20

# 获取数据最大同时传输数目
# 整数类型 默认为：20
SimultaneousTransfersForRetrieval = 20

# 实现存储订单精细控制的过滤器
# 执行命令的字符串 可选 （详情参见 https://lotus.filecoin.io/storage-providers/advanced-configurations/market/#using-filters-for-fine-grained-storage-and-retrieval-deal-acceptance）
Filter = ""

# 获取数据订单精细控制的过滤器
# 执行命令的字符串 可选
RetrievalFilter = ""

# 订单传输数据的存储位置
# 字符串类型 可选 为空值时默认使用`MARKET_REPO`的路径
TransfePath = ""

# 发送订单消息的最大费用
# FIL类型 默认为："0 FIL"
# FIL类型字符串形式为 整数+" FIL"
MaxPublishDealsFee = "0 FIL"

# 发送增加抵押消息时花费的最大费用
# FIL类型 默认为："0 FIL"
MaxMarketBalanceAddFee = "0 FIL"
```

## venus market 网络配置

这部分的配置决定了venus market 和外界交互的接口

### [API]
market 对外提供服务的接口

```
[API]
# Market 提供服务监听的地址
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

Market 在P2P网络中通信时使用的 通信地址
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

当market接入venus组件使用时，需要配置相关组件的API。

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

venus 提供签名服务的组件，它可以由两种类型：由venus-wallet直接提供的签名服务和由venus-gateway提供的间接签名服务

```
[Signer]
# 签名服务组件的类型
# 字符串类型  枚举："gateway"，"wallet"
Type = "gateway"

# 签名服务入口
# 字符串类型 必选（也可以直接通过命令行的 --signer-url flag 进行配置）
Url = "/ip4/192.168.200.128/tcp/45132/"

# venus 系列组件的鉴权token
# 字符串类型 必选（也可以直接通过命令行的 --auth-token flag 进行配置）
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


### [AuthNode]

venus 提供鉴权服务接入配置
```
[AuthNode]

# 鉴权服务入口
# 字符串类型 必选（也可以直接通过命令行的 --signer-url flag 进行配置）
Url = "http://192.168.200.128:8989"

# venus 系列组件的鉴权token
# 字符串类型 必选（也可以直接通过命令行的 --auth-token flag 进行配置）
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


## 矿工配置

预置矿工信息
```
[[StorageMiners]]
# 矿工的地址
# 字符串类型 必选
Addr =""

# 账户名
# 字符串类型 必选
Account = ""
```



## 数据库配置

Market 运行过程中产生的数据的存储数据库的设置
目前支持BadgerDB和MySQLDB，默认使用BadgerDB

### [Mysql]

MySQLDB的配置
```
[Mysql]

# 用于连接MySQL数据库的 connection string
# 字符串类型 如果要使用 MySQL 数据库的话，这是必选，否则使用默认的BadgerDB
ConnectionString = ""

# 打开连接的最大数量
# 整数类型 默认为100
MaxOpenConn = 100

# 空闲连接的最大数量
# 整数类型 默认为100
MaxIdleConn = 100

# 可复用连接的最大生命周期
# 时间字符串 默认为："1m"
# 时间字符串是由数字和时间单位组成的字符串，数字包括整数和小数，合法的单位包括 "ns", "us" (or "µs"), "ms", "s", "m", "h".
ConnMaxLifeTime = "1m"

# 是否输出数据库相关的调试信息
# 布尔值 默认false
Debug = false
```

##  扇区存储配置

配置 market导入数据后生成的扇区的存储空间
支持使用两种类型的数据存储方式： 文件系统存储和对象存储

### [[PieceStorage.Fs]]

配置本地文件系统作为扇区存储
对于大量数据的扇区，建议挂载和venus-sealer或者venus-cluster共用的文件系统进行配置 

```
[PieceStorage]
[[PieceStorage.Fs]]

# 存储空间的名称，它在market的所有的存储空间中，必须是唯一的
# 字符串类型 必选
Name = "local"

# 该存储空间是否可写（ read only false 即为可写）
# 布尔值 默认为 false
ReadOnly = false

# 该存储空间在本地文件系统中的路径
# 字符串类型 必选
Path = "/piecestorage/"

```

```
[PieceStorage]
[PieceStorage.S3]
# 存储空间的名称，它在market的所有的存储空间中，必须是唯一的
# 字符串类型 必选
Name:"s3"

# 该存储空间是否可写（ read only false 即为可写）
# 布尔值 默认为 false
ReadOnly = true

# 对象存储服务的入口
# 字符串类型 必选
EndPoint = "http://oss-cn-shanghai.aliyuncs.com/venus-market-test"

# 访问对象存储服务的参数
# 字符串类型 其中AccessKey，SecretKey必选，token 可选
AccessKey = "LTAI5t6HiFgsqN6eVJ......"
SecretKey = "AlFNH9NakUsVjVRxMHaaYP7p......"
Token = ""

```


## 日志设置
配置market 使用过程中，产生日志存储的位置

```
[Journal]

# 日志存储的位置
# 字符串类型 默认为："journal" (即`MARKET_REPO`文件夹下面的journal文件夹)
Path = "journal"
```

## 消息发送地址的配置

```
[AddressConfig]

# 是否降低使用woker地址发布消息的优先级，如果是，则只有在其他可选地址没有的情况下才会使用woker的地址发消息
# 布尔值 默认为 false
DisableWorkerFallback = false


[[DealPublishControl]]

# 发布订单消息的地址
# 字符串类型 必选
Addr = ""

# 持有相应地址的账户
# 字符串类型 必选
Account =""

```


## DAG存储设置

DAG 数据存储的配置

```
[DAGStore]

# DAG数据存储的根目录
# 字符串类型 默认为： "<MARKETS_REPO_PATH>/dagstore"
RootDir = "/root/.venusmarket/dagstore"

# 可以同时进行索引作业的最大数量
# 整数类型 默认为5 0表示不限制
MaxConcurrentIndex = 5

# 可以同时被捉取的最大未封装订单的数量
# 整数类型 默认为0 0表示不限制
MaxConcurrentReadyFetches = 0

# 可以被同时调用的存储API的最大数量
# 整数类型 默认为100
MaxConcurrencyStorageCalls = 100

# DAG 数据进行垃圾回收的时间间隔
# 时间字符串 默认为："1m0s"
# 时间字符串是由数字和时间单位组成的字符串，数字包括整数和小数，合法的单位包括 "ns", "us" (or "µs"), "ms", "s", "m", "h".
GCInterval = "1m0s"

# 临时文件的存储路径
# 字符串类型 可选 不设置则使用RooDir目录下的'transients'文件夹
Transient = ""

# 存储扇区索引数据的路径
# 字符串类型 可选 不设置则使用RooDir目录下的'index'文件夹
Index = ""

# 决定读取扇区数据时的时候是否跳过 Transient 中的文件，直接读取
# 布尔类型 默认为 false
UseTransient = false
```


## 数据索取配置

获取订单中存储的扇区数据时的相关配置

### [RetrievalPaymentAddress]
获取订单扇区数据时，使用的支付地址
```
[RetrievalPaymentAddress]
Addr = ""
Account = ""

```

### [RetrievalPricing]
获取订单扇区数据时，使用的定价策略

``` 
[RetrievalPricing]

# 使用的策略类型
# 字符串类型 可以选择"default"和"external"  默认为:"default"
# 前者使用内置的默认策略，后者使用外部提供的脚本自定义的策略
Strategy = "default"

[RetrievalPricing.Default]

# 对于经过认证的订单数据，是否定价为0
# 布尔值 默认为 "true"
# 只有Strategy = "default" 才会生效
VerifiedDealsFreeTransfer = true

[RetrievalPricing.External]
# 定义外部策略的脚本的路径
# 字符串类型 如果选择external策略时，必选
Path = ""
```
