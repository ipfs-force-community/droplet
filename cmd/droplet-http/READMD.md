## http 检索

支持通过 piece cid 检索，通过直接去 piecestore 查找和读取 piece，然后返回结果。

### 编译

```
make droplet-http
```

### 运行

> --droplet-repo-path 需要填写 `droplet` 的 `repo` 目录，方便从中获取 `piecestore` 的配置

```
./droplet-http run --listen 0.0.0.0:53241  --droplet-repo <path_to_droplet_repo>
```

还需要调整 `droplet` 配置文件 `config.toml` 中 `HTTPRetrievalMultiaddr` 字段的值，参考下面示例：

```toml
[CommonProvider]
  HTTPRetrievalMultiaddr = "/ip4/<ip>/tcp/53241/http"
```

> 上面配置中的 `ip` 是你本机的 IP 地址，`53241` 要确保和 `droplet-http` 使用的端口一致。

### TODO

[filplus 提出的 HTTP V2 检索要求](https://github.com/data-preservation-programs/RetrievalBot/blob/main/filplus.md#http-v2)
