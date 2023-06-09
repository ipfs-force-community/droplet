# Configurations of droplet

A typical `droplet` configuration looks like this:
```

# ****** Data transfer parameter configuration ***********
SimultaneousTransfersForStorage = 20
SimultaneousTransfersForStoragePerClient = 20
SimultaneousTransfersForRetrieval = 20


# ****** Global Basic Parameter Configuration ***********
[CommonProvider]
   ConsiderOnlineStorageDeals = true
   Consider OfflineStorageDeals = true
   ConsiderOnlineRetrievalDeals = true
   ConsiderOfflineRetrievalDeals = true
   ConsiderVerifiedStorageDeals = true
   ConsiderUnverifiedStorageDeals = true
   PieceCidBlocklist = []
   ExpectedSealDuration = "24h0m0s"
   MaxDealStartDelay = "336h0m0s"
   PublishMsgPeriod = "1h0m0s"
   MaxDealsPerPublishMsg = 8
   MaxProviderCollateralMultiplier = 2
   Filter = ""
   RetrievalFilter = ""
   TransferPath = ""
   MaxPublishDealsFee = "0 FIL"
   MaxMarketBalanceAddFee = "0 FIL"
   RetrievalPaymentAddress = ""
   DealPublishAddress = []
   [CommonProvider. RetrievalPricing]
     Strategy = "default"
     [CommonProvider. RetrievalPricing. Default]
       VerifiedDealsFreeTransfer = true
     [CommonProvider. RetrievalPricing. External]
       Path = ""
    

Each miner can have independent basic parameters. If there is no configuration, the global configuration will be used. The configuration options are as follows:

# ****** Miner Basic Parameter Configuration ********
[[Miners]]
   Addr = "f01000"
   Account = "testuser01"
  
    ConsiderOnlineStorageDeals = true
    ConsiderOfflineStorageDeals = true
    ConsiderOnlineRetrievalDeals = true
    ConsiderOfflineRetrievalDeals = true
    ConsiderVerifiedStorageDeals = true
    ConsiderUnverifiedStorageDeals = true
    PieceCidBlocklist = []
    ExpectedSealDuration = "24h0m0s"
    MaxDealStartDelay = "336h0m0s"
    PublishMsgPeriod = "1h0m0s"
    MaxDealsPerPublishMsg = 8
    MaxProviderCollateralMultiplier = 2
    Filter = ""
    RetrievalFilter = ""
    TransferPath = "/mnt/transfer"
    MaxPublishDealsFee = "0 FIL"
    MaxMarketBalanceAddFee = "0 FIL"
    RetrievalPaymentAddress = ""
    DealPublishAddress = []
    [CommonProvider. RetrievalPricing]
      Strategy = "default"
      [CommonProvider. RetrievalPricing. Default]
        VerifiedDealsFreeTransfer = true
      [CommonProvider. RetrievalPricing. External]
        Path = ""

# ****** droplet network configuration ***********
[API]
   ListenAddress = "/ip4/127.0.0.1/tcp/41235"
   RemoteListenAddress = ""
   Secret = "e647ee23cf95424162b974cd641b6a6479cbc7cb1209cc755f762c8248d50ba4"
   Timeout = "30s"

[Libp2p]
   ListenAddresses = ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]
   AnnounceAddresses = []
   NoAnnounceAddresses = []
   PrivateKey = "08011240d47934b6fccf8b79786335a55ccc04bdb9c92866cae2c0cea2fdefe0f2e7c18650dfbde5dd126c2a23a0d1c60686d3dedd064b67ba97c6161dd8007f0675e"


# ****** Venus Chain Service Configuration ***********
[Node]
   Url = "/ip4/192.168.200.151/tcp/3453"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdC11c2VyMDEiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.ETjNy3HMDS3ScZ3cax9xYb6AopNWYp4y71lZGCvYxMg"

[Messager]
   Url = "/ip4/127.0.0.1/tcp/39812"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdC11c2VyMDEiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.ETjNy3HMDS3ScZ3cax9xYb6AopNWYp4y71lZGCvYxMg"

[Signer]
   Type = "gateway"
   Url = "/ip4/127.0.0.1/tcp/45132"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdC11c2VyMDEiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.ETjNy3HMDS3ScZ3cax9xYb6AopNWYp4y71lZGCvYxMg"

[AuthNode]
   Url = "http://127.0.0.1:8989"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdC11c2VyMDEiLCJwZXJtIjoic2lnbiIsImV4dCI6IiJ9.ETjNy3HMDS3ScZ3cax9xYb6AopNWYp4y71lZGCvYxMg"



# ********** Database Settings ********
[Mysql]
ConnectionString = ""
MaxOpenConn = 100
MaxIdleConn = 100
ConnMaxLifeTime = "1m"
Debug = false


# ********* Sector Storage Setting ***********
[Piece Storage]
S3 = []

[[PieceStorage. Fs]]
Name = "local"
ReadOnly = false
Path = "./.vscode/test"


# *********** Log Settings ***********
[Journal]
Path = "journal"


# ********** DAG Storage Settings ********

[DAGStore]
RootDir = "/root/.droplet/dagstore"
MaxConcurrentIndex = 5
MaxConcurrentReadyFetches = 0
MaxConcurrencyStorageCalls = 100
GCInterval = "1m0s"
Transient = ""
Index = ""
Use Transient = false


# ********** Data Retrieval Configuration ********

RetrievalPaymentAddress = ""



# ****** Metric Configuration ***********
[Metrics]
   Enabled = false
   [Metrics. Exporter]
     Type = "prometheus"
     [Metrics. Exporter. Prometheus]
       RegistryType = "define"
       Namespace = ""
       EndPoint = "/ip4/0.0.0.0/tcp/4568"
       Path = "/debug/metrics"
       ReportingPeriod = "10s"
     [Metrics. Exporter. Graphite]
       Namespace = ""
       Host = "127.0.0.1"
       Port = 4568
       ReportingPeriod = "10s"

```

Next, using the above as the base, we will walk you through basic parameters, network configuration, Venus component configuration and other configuration options.

## Data Transfer Parameter Configuration

```
# Store the maximum number of simultaneous transfers for storage deals
# Integer type, default: 20
SimultaneousTransfersForStorage = 20

# The maximum number of simultaneous transfers of storage deal per client
# Integer type, default: 20
SimultaneousTransfersForStoragePerClient = 20

# The maximum number of simultaneous data transfers
# Integer type, default: 20
SimultaneousTransfersForRetrieval = 20
```

## Basic Parameter Configuration

The configuration mainly sets the preferences of how the market service should work. The functions of each configuration options are as follows:

```
# Whether to accept online storage deals
# Boolean value, defaults to true
ConsiderOnlineStorageDeals = true

# Whether to accept offline storage deals
# Boolean value, defaults to true
ConsiderOfflineStorageDeals = true

# Whether to accept online data retrieval requests
# Boolean value, defaults to true
ConsiderOnlineRetrievalDeals = true

# Decide whether to accept offline data retrieval requests
# Boolean value, defaults to true
ConsiderOfflineRetrievalDeals = true

# Whether to accept verified storage deals
# Boolean value, defaults to true
ConsiderVerifiedStorageDeals = true

# Whether to accept unverified storage deals
# Boolean value, defaults to true
ConsiderUnverifiedStorageDeals = true

# Storage deal data blacklist
# string array where each string is a CID, the default is empty
# CID data in the blacklist will not be used to fullfil deals
PieceCidBlocklist = []

# The maximum expected time for the storage deal to be sealed
# Time string, default: "24h0m0s"
# The time string is a string composed of numbers and time units. Numbers include integers and decimals. Legal units include "ns", "us" (or "µs"), "ms", "s", "m" , "h".
ExpectedSealDuration = "24h0m0s"

# Max delay before starting sealing a storage deal
# Time string, default: "336h0m0s"
MaxDealStartDelay = "336h0m0s"

# Periods between publishing messages
# Time string Default: "1h0m0s"
PublishMsgPeriod = "5m0s"

# The maximum number of deals in a publish message
# Integer type, default is 8
MaxDealsPerPublishMsg = 8

# Maximum storage provider collateral multiplication factor
# Integer type, default: 2
MaxProviderCollateralMultiplier = 2

# Filter storage deals through external executors, which can be executable programs or scripts
Filter = ""

# Filter retrieval requests through external executors, which can be executable programs or scripts
RetrievalFilter = ""

# Storage location of transferred deal data
# string type, optional, when it is empty, the path of `DROPLET_REPO` is used by default
TransferPath = ""

# Maximum fee for sending deal related messages
# FIL type, default: "0 FIL"
# The format of the FIL type string is integer + " FIL"
MaxPublishDealsFee = "0 FIL"

# The maximum fee spent when sending escrow related message
# FIL type, default: "0 FIL"
MaxMarketBalanceAddFee = "0 FIL"

# Reserved fields, this configuration option is invalid
[Retrieval Pricing]

# The type of strategy to use
# String type, you can choose "default" and "external", the default is: "default"
# The former uses the built-in default strategy, and the latter uses the strategy customized by the script provided externally
Strategy = "default"

[RetrievalPricing.Default]

# For verified storage deal, whether the price is 0
# boolean, defaults to "true"
# Only taking effects when Strategy = "default" 
VerifiedDealsFreeTransfer = true

[RetrievalPricing.External]
# Path to scripts that define external policies
# String type, Required if external strategy is selected
Path = ""

# This setting is a reserved field and is currently invalid
[AddressConfig]

# Whether to lower the priority of using the woker address to publish messages, if set to true, the woker address will be used to send messages only if there are no other applicable addresses
# Boolean, default is false
DisableWorkerFallback = false

[[AddressConfig. DealPublishControl]]

# Address to publish deal messages
# string type, required
Addr = ""

# The account holding the corresponding address
# string type, required
Account=""
```

## droplet Network Configuration

This part of the configuration determines the interface between the droplet and others

### [API]
The interface that market provides external services

```
[API]
# Market address where the service listens
# String type, required, default: "/ip4/127.0.0.1/tcp/41235"
ListenAddress = "/ip4/127.0.0.1/tcp/41235"

# reserved text
RemoteListenAddress = ""

# token used for encrypted communication
# String type, optional (automatically generated if there is none)
Secret = "878f9c1f88c6f68ee7be17e5f0848c9312897b5d22ff7d89ca386ed0a583da3c"

# reserved text
Timeout = "30s"
```

### [Libp2p]

The communication address used by the Market when communicating in the P2P network

```
[Libp2p]
# Listening network address
# string array, required default: ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]
ListenAddresses = ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]

# reserved text
AnnounceAddresses = []

# reserved text
NoAnnounceAddresses = []

# token used to generate peerid for p2p connections
# string, optional (automatically generated if not set)
PrivateKey = "08011240ae580daabbe087007d2b4db4e880af10d582215d2272669a94c49c854f36f99c35"
```

## Venus Chain Service Configuration

When accessing Venus Chain Service, the API of the relevant component needs to be configured.

### [Node]
Venus chain service access configuration

```
[Node]
# Chain service
# String type, Mandatory (can also be configured directly through the --node-url flag of the command line)
Url = "/ip4/192.168.200.128/tcp/3453"

# Authentication token of Venus chain service
# String type, Mandatory (can also be configured directly through the --auth-token flag of the command line)
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"

```


### [Messager]

venus message service access configuration

```
[Messager]
# Message service
# String type, optional (can also be configured directly through the --messager-url flag on the command line) It can be left blank when not connecting to the chain service
Url = "/ip4/192.168.200.128/tcp/39812/"

# Authentication token of Venus chain service 
# String type, optional (can also be configured directly through the --auth-token flag of the command line) It can be left blank when not connecting to the chain service
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


### [Signer]

Accessing Venus signature service, which can be of two types: signature services directly provided by venus-wallet and indirect signature services provided by sophon-gateway

```
[Signer]
# Type of signature service
# String type enumeration: "gateway", "wallet", "lotusnode"
Type = "gateway"

# Signature service
# String type, Mandatory (can also be configured directly through the --signer-url flag on the command line)
Url = "/ip4/192.168.200.128/tcp/45132/"

# Authentication token of Venus chain service
# String type, Mandatory (can also be configured directly through the --auth-token flag of the command line)
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


### [AuthNode]

Venus authentication service access configuration

```
[AuthNode]

# Authentication service
# String type， Optional (can also be configured directly through the --signer-url flag of the command line) It can be left blank when not connecting to the chain service
Url = "http://192.168.200.128:8989"

# Authentication token of Venus chain service
# String type, Optional (can also be configured directly through the --auth-token flag of the command line) It can be left blank when not connecting to the chain service
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


## Miner Configuration

Preset miner information
```
[[Miners]]
# Miner's address
# string type, required
Addr=""

# account name
# string type required
Account = ""

# Basic parameters, see above
```

:::tip

Basic parameters will use `CommonProvider` when not configured, which is as follows:
```
[[Miners]]
   Addr = "f02472"
   Account = "litao"
```

If one of the basic parameters is configured, then all items must be configured. For example:
```
[[Miners]]
   Addr = "f02472"
   Account = "litao"
   TransferPath = "/mnt/transfer/2472"
```
Such a configuration will cause other configuration items in the basic parameters to have zero values, instead of using the configuration in default `CommonProvider`.
For example, `ConsiderOnlineStorageDeals` corresponding to `f02472` will be equal to `false`, not `true` in `CommonProvider`.

At this point, it requires special attention. The correct configuration are as following:
```
[[Miners]]
   Addr = "f02472"
   Account = "litao"
   TransferPath = "/mnt/transfer/2472"
   ConsiderOnlineStorageDeals = true
   ConsiderOfflineStorageDeals = true
   ConsiderOnlineRetrievalDeals = true
   ConsiderOfflineRetrievalDeals = true
   ConsiderVerifiedStorageDeals = true
   ConsiderUnverifiedStorageDeals = true
   PieceCidBlocklist = []
   ExpectedSealDuration = "24h0m0s"
   MaxDealStartDelay = "336h0m0s"
   PublishMsgPeriod = "1m0s"
   MaxDealsPerPublishMsg = 8
   MaxProviderCollateralMultiplier = 2
   Filter = ""
   RetrievalFilter = ""
   MaxPublishDealsFee = "0 FIL"
   MaxMarketBalanceAddFee = "0 FIL"
   RetrievalPaymentAddress = ""
   [Retrieval Pricing]
     Strategy = "default"
     [RetrievalPricing. Default]
       VerifiedDealsFreeTransfer = true
     [RetrievalPricing.External]
       Path = ""
```

This is not very flexible, and will be considered for optimization in the future.

:::


## Database Configuration

The setting of the storage database for the data generated during the operation of the droplet.
BadgerDB and MySQLDB are currently supported, and BadgerDB is used by default.

### [Mysql]

MySQLDB configuration
```
[Mysql]

# The connection string used to connect to the MySQL database
# String type, If you want to use MySQL database, this is required, otherwise use the default BadgerDB
ConnectionString = ""

# Maximum number of open connections
# Integer type, defaults to 100
MaxOpenConn = 100

# Maximum number of idle connections
# Integer type, defaults to 100
MaxIdleConn = 100

# The maximum lifetime of a reusable connection
# time string, default: "1m"
# The time string is a string composed of numbers and time units. Numbers include integers and decimals. Legal units include "ns", "us" (or "µs"), "ms", "s", "m" , "h".
ConnMaxLifeTime = "1m"

# Whether to output database-related debugging information
# boolean default, false
Debug = false
```

## Sector Storage Configuration

Configure the storage space of imported data from droplet.
Two types of data storage are supported: file system storage or object storage.

### [[PieceStorage. Fs]]

Configure the local file system as sector storage
For sectors with a large amount of data, it is recommended to mount the file system shared with sophon-cluster

```
[Piece Storage]
[[PieceStorage. Fs]]

# The name of the storage space, which must be unique among all storage spaces in the market
# string type, required
Name = "local"

# Whether the storage space is writable (readOnly=false means writable)
# boolean, default is false
ReadOnly = false

# The path of the storage space in the local file system
# string type, required
Path = "/piecestorage/"

```

```
[Piece Storage]
[[PieceStorage.S3]]
# The name of the storage space, which must be unique among all storage spaces in the market
# string type, required
Name = "s3"

# Whether the storage space is writable (readOnly=false means writable)
# boolean, default is false
ReadOnly = true

# Object storage service
# string type, required
# Support individual EndPoint ("oss-cn-shanghai.aliyuncs.com") or complete EndPoint Url ("http://oss-cn-shanghai.aliyuncs.com")
EndPoint = "oss-cn-shanghai.aliyuncs.com"

# Bucket name of the object storage service
# string type, required
Bucket = "droplet"

# Specify the subdirectory in the Bucket
# string type, optional
SubDir = "dir1/dir2"

# Access the parameters of the object storage service
# String type, AccessKey and SecretKey are mandatory, and Token is optional
AccessKey = "LTAI5t6HiFgsqN6eVJ..."
SecretKey = "AlFNH9NakUsVjVRxMHaaYP7p..."
Token = ""

```


## Log Settings
Configure the location where the log is stored during the use of the market.

```
[Journal]

# The location of the log storage
# String type, The default is: "journal" (that is, the journal folder under the `DROPLET_REPO` folder)
Path = "journal"
```


## DAG Storage Settings

Configuration of the DAG datastore.

```
# Refer to github.com/filecoin-project/dagstore/dagstore.go
[DAGStore]

# The root directory of the DAG data store
# String type, Default: "<DROPLET_REPO_PATH>/dagstore"
RootDir = "/root/.droplet/dagstore"

# The maximum number of index jobs that can be performed at the same time
# Integer type, defaults to 5, 0 means unlimited
MaxConcurrentIndex = 5

# The maximum number of unsealed deals that can be fetched at the same time
# Integer type, defaults to 0, 0 means unlimited
MaxConcurrentReadyFetches = 0

# The maximum number of storage APIs that can be called simultaneously
# Integer type, defaults to 100
MaxConcurrencyStorageCalls = 100

# DAG data garbage collection interval
# time string, default: "1m0s"
# The time string is a string composed of numbers and time units. Numbers include integers and decimals. Legal units include "ns", "us" (or "µs"), "ms", "s", "m" , "h".
GCInterval = "1m0s"

# Storage path for temporary files
# string type, optional, if not set, the 'transients' folder in the RooDir directory will be used
Transient = ""

# Path to store sector index data
# String type, optional, if not set, the 'index' folder under the RooDir directory will be used
Index = ""

# Do not use local cache, read data source directly
# Boolean type, defaults to false
Use Transient = false
```


## Data Retrieval

Relevant configuration when obtaining the sector data stored in the deal

### [RetrievalPaymentAddress]
The receiving address used for retrieval requests
```
RetrievalPaymentAddress = ""
```

## Metric Configuration

Configure Metric-related parameters.


```toml
[Metrics]

# Whether to enable Metric
# boolean, default is false
Enabled = false

# Metric export settings
[Metrics. Exporter]

# The type of Metric export
# String type, Optional values are "prometheus" and "graphite" Default is "prometheus"
Type = "prometheus"

# Prometheus export settings
[Metrics.Exporter.Prometheus]

# type of register
# String type, Optional, values are "define" and "default"; Default is "define"
# define: use new registry; default: use default registry provided by Prometheus
RegistryType = "define"

# Namespaces
# string type, defaults to ""
Namespace = ""

# listen address
# String type, Default is "/ip4/0.0.0.0/tcp/4568"
EndPoint = "/ip4/0.0.0.0/tcp/4568"

# The access path of the Metrics indicator
# string type, defaults to "/debug/metrics"
Path = "/debug/metrics"

# Metric index aggregation cycle
# time string, defaults to "10s"
ReportingPeriod = "10s"


# Graphite export settings
[Metrics. Exporter. Graphite]

# Namespaces
# string type, defaults to ""
Namespace = ""

# listen address
# String type, Default is "127.0.0.1"
Host = "127.0.0.1"

# Listen port
# integer type, default is 4568
Port = 4568

# Metric index aggregation cycle
# time string, defaults to "10s"
ReportingPeriod = "10s"
```