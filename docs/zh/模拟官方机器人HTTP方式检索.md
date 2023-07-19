# 模拟官方机器人 HTTP 方式检索

目前常见的检索方式 `GraphSync`、`HTTP`、`Bitswap` 三种，`droplet` 默认使用的是 `GraphSync` 方式，在 v2.8.0 版本之后，也支持 `HTTP` 方式。由于 `HTTP` 是无状态的请求数据，在检索时无需要建立大量索引的特性，对于检索成功率提升较高。



## 开启 HTTP 检索

除了需要将 Libp2p 的端口公网映射出去，用于 `GraphSync` 接单和检索使用；droplet 还需要将 `ListenAddress` 的 **41235** 端口映射出去 (可以自定义，在配置文件中修改即可)，用于 `HTTP` 方式的检索使用。

在 `.droplet/config.toml` 中，配置 **HTTPRetrievalMultiaddr** 的公网地址，用于向检索的客户端发送订单的 piece 数据。

```
[CommonProvider]
  HTTPRetrievalMultiaddr = "/ip4/公网IP地址/tcp/41235/http"
  ConsiderOnlineStorageDeals = false
  ConsiderOfflineStorageDeals = false
  ConsiderOnlineRetrievalDeals = true
  ConsiderOfflineRetrievalDeals = true
```



### 通过 HTTP 请求验证检索功能

```bash
curl http://公网IP:41235/piece/baga6ea4sexxxxxx --output /tmp/test
```

如果可以正常下载到 piece，则说明 HTTP 方式的检索配置成功



### 通过 RetrievalBot 工具验证检索功能

```bash
git clone https://github.com/simlecode/RetrievalBot.git
git checkout feat/simple-http
make
```



1. 确保 droplet 已经开启 HTTP 检索；
2. 配置 RetrievalBot。 先通过 `droplet actor info --miner f0xxxx` 获取 RetrievalBot 工具需要的 **PeerID** 和 **Multiaddrs**。

RetrievalBot 配置文件示例如下:

```toml
# http_retrieval.toml

# miner id
ID = "f0xxxx"
# miner peer
PeerID = "12D3KooWBvPWkWLEHbr7iwDUs8CMQ8j2V85keakBZunP3YMZ9SEk"
#
Multiaddrs = ["/ip4/1.182.90.10/tcp/48027"]
# piece cids
Pieces = [
    "baga6ea4seaqd65uw3tksjc5nilba5fmy4swlbchwx4k47cpe3slba37z7o26cga",
]
```

使用 `./http_worker` 进行验证

返回值为 `miner f0xxx retrieval bagaxxxxxoa success` 则说明支持 HTTP 方式检索 
