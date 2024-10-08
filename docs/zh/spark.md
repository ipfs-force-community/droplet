## Spark

### 开启

1. 打开 `droplet` 配置文件 `config.toml`，在 `[CommonProvider]` 下面增加下面的配置，`PublicHostname` 是公网 IP 地址。

```
  [CommonProvider.IndexProvider]
    Enable = true
    EntriesCacheCapacity = 1024
    EntriesChunkSize = 16384
    TopicName = ""
    PurgeCacheOnStart = false
    WebHost = "cid.contact"
    DataTransferPublisher = true
    [CommonProvider.IndexProvider.Announce]
      AnnounceOverHttp = true
      DirectAnnounceURLs = ["https://cid.contact/ingest/announce"]
    [CommonProvider.IndexProvider.HttpPublisher]
      Enabled = true
      PublicHostname = "127.0.0.1"
      Port = 41263
      WithLibp2p = true
```

2. 调整 libp2p 配置

```
[Libp2p]
  ListenAddresses = ["/ip4/0.0.0.0/tcp/58418"]
  AnnounceAddresses = ["/ip4/<YOUR_PUBLIC_IP_ADDRESS>/tcp/58418"]
```

### 命令行命令

1. 发布单个矿工所有的订单到 ipni

```bash
./droplet index announce-all --miner t01001
```

2. 发布单个订单到 ipni

```bash
./droplet index announce-deal <deal uuid> 
or
./droplet index announce-deal <proposal cid>
```

3. 从 ipni 移除订单

```bash
./droplet index announce-remove-deal <deal uuid> 
or
./droplet index announce-remove-deal <proposal cid>
```

### 检查

1. 通过 https://cid.contact/ 输入订单的 `payload cid`，查看订单是否已经发布到 ipni。
2. 通过 miner peerid 来检查是否已注册到 ipni，https://cid.contact/providers/12D3KooWLR8GZNKpN7zM6T24zw1z4gQrCdk2tFWzG8mfFaaszGay

### 注意

`droplet` 版本要不低于 `v2.12.0`，`venus` 版本要不低于 `v1.16.0`。
