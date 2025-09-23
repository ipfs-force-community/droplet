# Simulate Official Robot HTTP Retrieval Method

Currently, the common retrieval methods are `GraphSync`, `HTTP`, and `Bitswap`. `droplet` defaults to using the `GraphSync` method, but starting from version v2.8.0, it also supports the `HTTP` method. Since `HTTP` is a stateless request for data and does not require establishing a large number of indexes during retrieval, it significantly improves the retrieval success rate.

## Enable HTTP Retrieval

In addition to mapping the Libp2p port to the public network for `GraphSync` deal acceptance and retrieval, `droplet` also needs to map the `ListenAddress` **41235** port to the public network (this can be customized by modifying it in the configuration file) for HTTP-based retrieval.

In `.droplet/config.toml`, configure the public network address for **HTTPRetrievalMultiaddr** to send the piece data of the deal to the retrieving client.

```
[CommonProvider]
  HTTPRetrievalMultiaddr = "/ip4/public IP address/tcp/41235/http"
  ConsiderOnlineStorageDeals = false
  ConsiderOfflineStorageDeals = false
  ConsiderOnlineRetrievalDeals = true
  ConsiderOfflineRetrievalDeals = true
```

### Verify Retrieval Functionality via HTTP Request

```bash
curl http://public IP:41235/piece/baga6ea4sexxxxxx --output /tmp/test
```

If the piece can be downloaded normally, it indicates that the HTTP retrieval configuration is successful.

### Verify Retrieval Functionality via RetrievalBot Tool

```bash
git clone https://github.com/simlecode/RetrievalBot.git
git checkout feat/simple-http
make
```

1. Ensure that `droplet` has enabled HTTP retrieval;
2. Configure RetrievalBot. First, obtain the **PeerID** and **Multiaddrs** required by the RetrievalBot tool via `droplet actor info --miner f0xxxx`.

RetrievalBot configuration file example:

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

Use `./http_worker` for verification.

If the return value is `miner f0xxx retrieval bagaxxxxxoa success`, it indicates support for HTTP retrieval.
