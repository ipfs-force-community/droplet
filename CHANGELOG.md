# droplet changelog

## v2.8.3
* fix(release deals): skip update state if deal expired [[#443](https://github.com/ipfs-force-community/droplet/pull/443)]

## v2.8.2

* feat: support filter deals by `SectorExpiration` [[#404](https://github.com/ipfs-force-community/droplet/pull/404)]
* fix: import deals slow [[#371](https://github.com/ipfs-force-community/droplet/pull/371)]


## v2.8.1

* feat: update gateway deps by @LinZexiao in https://github.com/ipfs-force-community/droplet/pull/362

## v2.8.0

* fix: repo compatibility for cli [[#348](https://github.com/ipfs-force-community/droplet/pull/348)]
* chore: output piece size to log [[#351](https://github.com/ipfs-force-community/droplet/pull/351)]
* fix: use old client repo when create market client[[#353](https://github.com/ipfs-force-community/droplet/pull/353)]

## v2.8.0-rc1

* feat: set address.CurrentNetwork when launch up [[#321](https://github.com/ipfs-force-community/droplet/pull/321)]
* opt: not wait for index results to be generated [[#324](https://github.com/ipfs-force-community/droplet/pull/324)]
* feat: output more power info [[#328](https://github.com/ipfs-force-community/droplet/pull/328)]
* fix: adapt deal filter format to CIDgravity / 修改 dealfilter 的实现, 以兼容 CIDgravity [[#329](https://github.com/ipfs-force-community/droplet/pull/329)]
* opt: ensure the type of signature data [[#330](https://github.com/ipfs-force-community/droplet/pull/330)]
* Feat: unseal piece before tansfer / 在数据传输之前先 unseal piece 数据 [[#331](https://github.com/ipfs-force-community/droplet/pull/331)]
* chore(deps): bump github.com/gin-gonic/gin from 1.9.0 to 1.9.1 [[#332](https://github.com/ipfs-force-community/droplet/pull/332)]
* feat: batch send deals [[#297](https://github.com/ipfs-force-community/droplet/pull/297)]
* feat: replace market with droplet [[#334](https://github.com/ipfs-force-community/droplet/pull/334)]
* feat: import deals [[#335](https://github.com/ipfs-force-community/droplet/pull/335)]

## v2.7.2

* 修复订单卡在 StorageDealStaged 状态 [[#324](https://github.com/ipfs-force-community/droplet/pull/324)]

## v2.7.1

* update ffi

## v2.7.0

* update venus dependency to v1.11.0
* bump up version to v2.7.0

## v2.7.0-rc1

### New Features

* feat: add status api to detect api ready by @hunjixin /添加状态检测接口 [[#282](https://github.com/ipfs-force-community/droplet/pull/282)]
* feat: Add miner manager related commands @diwufeiwen /增加 miner manager 相关命令 [[#293](https://github.com/ipfs-force-community/droplet/pull/293)] 
* feat: add command to print signer deal detail by @simlecode /增加两个命令行用于输出单个存储订单和检索的详情 [[#298](https://github.com/ipfs-force-community/droplet/pull/298)]
* feat: unsealed from sp through sophon-gateway / 通过sophon-gateway给SP下发unsealed请求 by @diwufeiwen [[#267](https://github.com/ipfs-force-community/droplet/pull/267)]
* feat: opt deal list cmds by @simlecode / 订单查询优化 [[#301](https://github.com/ipfs-force-community/droplet/pull/301)] 
* feat: new api ReleaseDeals by @0x5459 /新增 ReleaseDeals 接口 [[#305](https://github.com/ipfs-force-community/droplet/pull/305)]
* feat: add command to cancel data transfer by @simlecode / 根据条件取消检索订单 [[#307](https://github.com/ipfs-force-community/droplet/pull/307)]
* feat: more time for query retrieval ask by @hunjixin /querytimeout从5秒改成5分钟[[#304](https://github.com/ipfs-force-community/droplet/pull/304)]
* feat: destroy shards by @simlecode /添加DagstoreDestroyShard命令 [[#309](https://github.com/ipfs-force-community/droplet/pull/309)] 
* feat: update unseal api / 更新 unseal 的接口 by @LinZexiao [[#314](https://github.com/ipfs-force-community/droplet/pull/314)]
* feat: add docker push by @hunjixin /增加推送到镜像仓库的功能 [[#316](https://github.com/ipfs-force-community/droplet/pull/316)]
* feat: add command to expend claim term by @simlecode /添加续期命令 [[#315](https://github.com/ipfs-force-community/droplet/pull/315)] 


### Bug Fixes
* fix: add composite indexes by @simlecode / 添加联合索引 [[#294](https://github.com/ipfs-force-community/droplet/pull/294)]
* fix: check deal state in ReleaseDeals by @0x5459 / ReleaseDeals 方法需要判断订单状态 [[#308](https://github.com/ipfs-force-community/droplet/pull/308)]
* fix: add check for miner config by @simlecode / 未找到矿工配置时返回错误 [[#311](https://github.com/ipfs-force-community/droplet/pull/311)]
* fix: Circular search for deals by @simlecode / 循环检索订单 [[#310](https://github.com/ipfs-force-community/droplet/pull/310)]
* fix: PaymentAddress uses a fake address when retrieval data does not exist by @simlecode / 当检索数据不存在时，paymentaddress用个假地址 [[#312](https://github.com/ipfs-force-community/droplet/pull/312)]

## v2.6.0

* 增加列出 storage/retrieval asks 命令行 [[#272](https://github.com/ipfs-force-community/droplet/pull/272)]
* 重构 updatedealstatus 接口 [[#289](https://github.com/ipfs-force-community/droplet/pull/289)]
* 升级 venus、sophon-messager、sophon-gateway 和 sophon-auth 版本到 v1.10.0
* 升级 go-jsonrpc 版本到 v0.1.7
