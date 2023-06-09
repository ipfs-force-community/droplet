# Droplet Client Configurations

A typical configuration of `droplet-client` is as follows...

```

Simultaneous Transfers For Retrieval = 20
Simultaneous Transfers For Storage = 20

[Default MarketAddress]
   Addr = "t3qkgm5h7nmusacfggd744w7fdj45rn6iyl7n6s6lr34t4qlfebiphmm3vxtwc4a4acqi4nv3pqk6h7ddqqz5q"
   Account = ""

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

Among them, it can be divided into three parts: 1) client network configuration, 2) configuration of Venus chain service components and 3) other configurations

## DROPLET Client network configuration

This part of the configuration determines the interface between the `droplet client` and external actors

### [API]

This section defines the external interface of `droplet-client`

```
[API]
# droplet-client provides the address where the service listens
# String type, required, default: "/ip4/127.0.0.1/tcp/41235"
ListenAddress = "/ip4/127.0.0.1/tcp/41235"

# reserved text
RemoteListenAddress = ""

# key used for encrypted communication
# String type, optional, automatically generated none is supplied
Secret = "878f9c1f88c6f68ee7be17e5f0848c9312897b5d22ff7d89ca386ed0a583da3c"

# reserved text
Timeout = "30s"
```

### [Libp2p]

The communication address used for P2P network

```
[Libp2p]
# Listening network address
# string array, required, default: ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]
ListenAddresses = ["/ip4/0.0.0.0/tcp/58418", "/ip6/::/tcp/0"]

# reserved text
AnnounceAddresses = []

# reserved text
NoAnnounceAddresses = []

# Private key for p2p encrypted communication
# string, optional, automatically generated if none is supplied
PrivateKey = "08011240ae580daabbe087007d2b4db4e880af10d582215d2272669a94c49c854f36f99c35"
```


## Venus Chain Service Configuration

When the `droplet-client` is connected to the `venus components`, the API of the related component needs to be configured.

### [Node]

Venus chain service access configuration

```
[Node]
# Entrance of the chain service
# String type, mandatory, can also be configured directly through the --node-url flag of the command line
Url = "/ip4/192.168.200.128/tcp/3453"

# Authentication token of Venus series components
# String type, mandatory, can also be configured directly through the --auth-token flag of the command line
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"

```


### [Messager]

`sophon-messager` service access configuration

```
[Messager]
# Message service entry
# String type, mandatory, can also be configured directly through the --messager-url flag on the command line
Url = "/ip4/192.168.200.128/tcp/39812/"

# Authentication token of venus series components
# String type, mandatory, can also be configured directly through the --auth-token flag of the command line
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


### [Signer]

The Venus component that provide signing services
Only signature services of `wallet` type can be used in `droplet-client`

```
[Signer]
# Type of signature service component
# String type can only be "wallet"
Type = "wallet"

# Signature service entry
# String type, mandatory, can also be configured directly through the --signer-url flag on the command line
Url = "/ip4/192.168.200.128/tcp/5678/"

# wallet token for authentication
# string type, mandatory
Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZm9yY2VuZXQtbnYxNiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.PuzEy1TlAjjNiSUu_tbHi2XPUritDLm9Xf5UW3MHRe8"
```


## Other configuration

```
# Get the maximum number of retrieval request for simultaneous transfers
# Integer type Default: 20
Simultaneous Transfers For Retrieval = 20

# Store the maximum number of simultaneous transfers of storage deals
# Integer type Default: 20
Simultaneous Transfers For Storage = 20

# The default address of the current droplet-client
# String type, optional, can also be configured directly through the --addr flag of the command line
DefaultMarketAddress = "t3qkgm5h7nmusacfggd744w7fdj45rn6iyl7n6s6lr34t4qlfebiphmm3vxtwc4a4acqi4nv3pqk6h7ddqqz5q:username"
```