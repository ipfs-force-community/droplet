# venus-market changelog

## v2.7.0-rc1

### New Features

* feat: add status api to detect api ready by @hunjixin /添加状态检测接口 [[#282](https://github.com/filecoin-project/venus-market/pull/282)]
* feat: Add miner manager related commands @diwufeiwen /增加 miner manager 相关命令 [[#293](https://github.com/filecoin-project/venus-market/pull/293)] 
* feat: add command to print signer deal detail by @simlecode /增加两个命令行用于输出单个存储订单和检索的详情 [[#298](https://github.com/filecoin-project/venus-market/pull/298)]
* feat: unsealed from sp through venus-gateway / 通过venus-gateway给SP下发unsealed请求 by @diwufeiwen [[#267](https://github.com/filecoin-project/venus-market/pull/267)]
* feat: opt deal list cmds by @simlecode / 订单查询优化 [[#301](https://github.com/filecoin-project/venus-market/pull/301)] 
* feat: new api ReleaseDeals by @0x5459 /新增 ReleaseDeals 接口 [[#305](https://github.com/filecoin-project/venus-market/pull/305)]
* feat: add command to cancel data transfer by @simlecode / 根据条件取消检索订单 [[#307](https://github.com/filecoin-project/venus-market/pull/307)]
* feat: more time for query retrieval ask by @hunjixin /querytimeout从5秒改成5分钟[[#304](https://github.com/filecoin-project/venus-market/pull/304)]
* feat: destroy shards by @simlecode /添加DagstoreDestroyShard命令 [[#309](https://github.com/filecoin-project/venus-market/pull/309)] 
* feat: update unseal api / 更新 unseal 的接口 by @LinZexiao [[#314](https://github.com/filecoin-project/venus-market/pull/314)]
* feat: add docker push by @hunjixin /增加推送到镜像仓库的功能 [[#316](https://github.com/filecoin-project/venus-market/pull/316)]
* feat: add command to expend claim term by @simlecode /添加续期命令 [[#315](https://github.com/filecoin-project/venus-market/pull/315)] 


### Bug Fixes
* fix: add composite indexes by @simlecode / 添加联合索引 [[#294](https://github.com/filecoin-project/venus-market/pull/294)]
* fix: check deal state in ReleaseDeals by @0x5459 / ReleaseDeals 方法需要判断订单状态 [[#308](https://github.com/filecoin-project/venus-market/pull/308)]
* fix: add check for miner config by @simlecode / 未找到矿工配置时返回错误 [[#311](https://github.com/filecoin-project/venus-market/pull/311)]
* fix: Circular search for deals by @simlecode / 循环检索订单 [[#310](https://github.com/filecoin-project/venus-market/pull/310)]
* fix: PaymentAddress uses a fake address when retrieval data does not exist by @simlecode / 当检索数据不存在时，paymentaddress用个假地址 [[#312](https://github.com/filecoin-project/venus-market/pull/312)]

## v2.6.0

* 增加列出 storage/retrieval asks 命令行 [[#272](https://github.com/filecoin-project/venus-market/pull/272)]
* 重构 updatedealstatus 接口 [[#289](https://github.com/filecoin-project/venus-market/pull/289)]
* 升级 venus、venus-messager、venus-gateway 和 venus-auth 版本到 v1.10.0
* 升级 go-jsonrpc 版本到 v0.1.7
