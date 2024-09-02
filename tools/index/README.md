# 索引工具

主要有两个功能，一个是给未生成索引的 active 订单生成索引，另一个是迁移 top index 到 MongoDB，迁移 shard 到 MySQL。

## 编译

```bash
make index
```

## 生成索引

先去 droplet 获取订单状态是 active 的订单，然后去遍历 car 文件，如果被 active 订单使用且未生成索引，则为其生成索引。

* --car-dir：存储 car 文件的目录，需要用绝对路径。
* --index-dir：存储索引文件的目录，需要用绝对路径，`droplet` 默认在 `~/.droplet/dagstore/index`。
* --mongo-url：MongoDB 的连接地址，用于存储 top index，数据库是 `market_index`，collection 是 `top_index`。
* --mysql-url：MySQL 的连接地址，用于存储 shard 状态，要和 `droplet` 使用同一个数据库，表名是 `shards`。
* --droplet-url：droplet 服务的 RPC 地址。
* --droplet-token：droplet 服务的 token。
* --start：订单创建时间需大于设置的值。
* --end：订单创建时间需小于设置的值。
* --concurrency：生成索引的并发数，默认是 1。
* --miner-addr：指定 miner 生成索引，未设置则给所有 miner 生成索引。

```bash
./index-tool gen-index \
--car-dir=<car dir> \
--index-dir=<index dir> \
--mongo-url="mongodb://user:pass@host/?retryWrites=true&w=majority" \
--mysql-url="user:pass@(127.0.0.1:3306)/venus-market?parseTime=true&loc=Local" \
--droplet-urls="/ip4/127.0.0.1/tcp/41235" \
--droplet-token=<token>
```

> 成功生成索引会输出类似日志：`generate index success: xxxxxx`

## 迁移索引

目前 top index 和 shard 都是存储在 badger，这样多个 droplet 时不能共享，所有需要把 top index 存储到 MongoDB，shard 存储到 MySQL，方便共享数据。

* --index-dir：存储索引文件的目录，`droplet` 默认在 `~/.droplet/dagstore/index`。
* --mongo-url：MongoDB 的连接地址，用于存储 top index，数据库是 `market_index`，`collection` 是 `top_index`。
* --mysql-url：MySQL 的连接地址，用于存储 shard 状态，要和 `droplet` 使用同一个数据库，表名是 `shards`。

```bash
./index-tool migrate-index \
--index-dir=<index dir> \
--mongo-url="mongodb://user:pass@host/?retryWrites=true&w=majority" \
--mysql-url="user:pass@(127.0.0.1:3306)/venus-market?parseTime=true&loc=Local" \
--droplet-urls="/ip4/127.0.0.1/tcp/41235" \
--droplet-token=<token>
```

> 成功迁移索引会输出类似日志：`migrate xxxxx success`
