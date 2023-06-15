## http 检索

支持通过 piece cid 检索，通过直接去 piecestore 查找和读取 piece，然后返回结果。

### 配置

需要调整 `droplet` 配置文件 `config.toml` 中 `HTTPRetrievalMultiaddr` 字段的值，参考下面示例：

```toml
[CommonProvider]
  HTTPRetrievalMultiaddr = "/ip4/<ip>/tcp/41235/http"
```

> 上面配置中的 `ip` 是你本机的 IP 地址，`41235` 要确保和 `droplet` 使用的端口一致。

### TODO

[filplus 提出的 HTTP V2 检索要求](https://github.com/data-preservation-programs/RetrievalBot/blob/main/filplus.md#http-v2)
