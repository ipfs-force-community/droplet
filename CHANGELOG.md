# droplet changelog

## v2.11.0-rc1

* feat: import data by uuid [[#471](https://github.com/ipfs-force-community/droplet/pull/471)]
* fix: use the received deal uuid [[#472](https://github.com/ipfs-force-community/droplet/pull/472)]
* fix: peer id is nil [[#474](https://github.com/ipfs-force-community/droplet/pull/474)]
* fix: clean blockstore after retrieval completed [[#477](https://github.com/ipfs-force-community/droplet/pull/477)]
* fix: set deal id when import deal from boost [[#480](https://github.com/ipfs-force-community/droplet/pull/480)]
* chore: merge release v2.10 to master [[#490](https://github.com/ipfs-force-community/droplet/pull/490)]
* feat: retrieval trustless [[#486](https://github.com/ipfs-force-community/droplet/pull/486)]
* feat: add regular file check [[#492](https://github.com/ipfs-force-community/droplet/pull/492)]
* feat: retrieval padding piece [[#491](https://github.com/ipfs-force-community/droplet/pull/491)]
* fix: disable auth when off chain service [[#493](https://github.com/ipfs-force-community/droplet/pull/493)]
* opt: split into droplet and droplet-client [[#494](https://github.com/ipfs-force-community/droplet/pull/494)]
* feat: get deal by deal id  [[#495](https://github.com/ipfs-force-community/droplet/pull/495)]
* Feat/ouput deal with json format [[#497](https://github.com/ipfs-force-community/droplet/pull/497)]
* chore: update doc about retrieval [[#498](https://github.com/ipfs-force-community/droplet/pull/498)]
* Feat/add more metrics [[#499](https://github.com/ipfs-force-community/droplet/pull/499)]
* opt: cmd: adjust display information [[#500](https://github.com/ipfs-force-community/droplet/pull/500)]
* feat: piecestorage support recursive lookup file [[#501](https://github.com/ipfs-force-community/droplet/pull/501)]
* chore(deps): bump golang.org/x/crypto from 0.14.0 to 0.17.0 [[#505](https://github.com/ipfs-force-community/droplet/pull/505)]
* Feat/start dagstore in goroutine [[#506](https://github.com/ipfs-force-community/droplet/pull/506)]
* test: Querying files from a large number of files takes time [[#507](https://github.com/ipfs-force-community/droplet/pull/507)]
* chore(deps): bump github.com/quic-go/quic-go from 0.38.1 to 0.38.2 [[#508](https://github.com/ipfs-force-community/droplet/pull/508)]
* feat: implement direct deal [[#509](https://github.com/ipfs-force-community/droplet/pull/509)]

## v2.10.0

## v2.10.0-rc6

* chore: update venus to v1.14.0-rc6
* fix: Paid retrieval failed [[#484](https://github.com/ipfs-force-community/droplet/pull/484)]


## v2.10.0-rc5

* chore: update venus to v1.14.0-rc4

## v2.10.0-rc4

* fix: set deal id when import deal from boost [[#480](https://github.com/ipfs-force-community/droplet/pull/480)]
* feat: import deal data by uuid [[#471](https://github.com/ipfs-force-community/droplet/pull/471)]

## v2.10.0-rc3

* fix: use the received deal uuid [[#473](https://github.com/ipfs-force-community/droplet/pull/473)]
* fix: clean blockstore after retrieval completed [[#478](https://github.com/ipfs-force-community/droplet/pull/478)]

## v2.10.0-rc2

* feat/use deal bound from policy directly [[#467](https://github.com/ipfs-force-community/droplet/pull/467)]
* fix: check deal end epoch with DealMaxDuration [[#468](https://github.com/ipfs-force-community/droplet/pull/468)]
* chore: update go-jsonrpc v0.1.8 [[#469](https://github.com/ipfs-force-community/droplet/pull/469)]

## v2.10.0-rc1

* fix: remove trace goroutine for dagstore wrapper [[#419](https://github.com/ipfs-force-community/droplet/pull/419)]
* feat: handle PreCommitSectorBatch2 message [[#420](https://github.com/ipfs-force-community/droplet/pull/420)]
* fix: do not cover token of node, auth, messager [[#424](https://github.com/ipfs-force-community/droplet/pull/424)]
* doc: update doc about unit entry [[#426](https://github.com/ipfs-force-community/droplet/pull/426)]
* Chore/merge release v2.9 [[#428](https://github.com/ipfs-force-community/droplet/pull/428)]
* fix: payload size is 0 when generate index [[#429](https://github.com/ipfs-force-community/droplet/pull/429)]
* docs: update quick start doc [[#430](https://github.com/ipfs-force-community/droplet/pull/430)]
* feat: generate manifest by piece file [[#431](https://github.com/ipfs-force-community/droplet/pull/431)]
* feat: support storage deal protocol v2 [[#435](https://github.com/ipfs-force-community/droplet/pull/435)]
* feat: support deal status protocol [[#436](https://github.com/ipfs-force-community/droplet/pull/436)]
* feat: list deal descending by default [[#440](https://github.com/ipfs-force-community/droplet/pull/440)]
* fix(release deals): skip update state if deal expired [[#443](https://github.com/ipfs-force-community/droplet/pull/443)]
* fix: generate manifest [[#445](https://github.com/ipfs-force-community/droplet/pull/445)]
* Update 快速启用.md [[#446](https://github.com/ipfs-force-community/droplet/pull/446)]
* fix: add timeout for query ask [[#449](https://github.com/ipfs-force-community/droplet/pull/449)]
* Update 批量发单.md [[#450](https://github.com/ipfs-force-community/droplet/pull/450)]
* opt: support offline compute commp [[#451](https://github.com/ipfs-force-community/droplet/pull/451)]
* feat: filter retrieval by deal piece [[#454](https://github.com/ipfs-force-community/droplet/pull/454)]
* fix: Do not recover failed indexes on startup [[#453](https://github.com/ipfs-force-community/droplet/pull/453)]
* add filter Example [[#455](https://github.com/ipfs-force-community/droplet/pull/455)]
* fix: could not judge offline deal from boost [[#458](https://github.com/ipfs-force-community/droplet/pull/458)]
* opt: piece storage supports files with .car suffix [[#459](https://github.com/ipfs-force-community/droplet/pull/459)]
* chore(deps): bump golang.org/x/net from 0.11.0 to 0.17.0 [[#460](https://github.com/ipfs-force-community/droplet/pull/460)]
* fix docs droplet actor-funds add command [[#461](https://github.com/ipfs-force-community/droplet/pull/461)]

## v2.9.0

## v2.9.0-rc1

* Update README.md [[#341](https://github.com/ipfs-force-community/droplet/pull/341)]
* doc: fix link [[#349](https://github.com/ipfs-force-community/droplet/pull/349)]
* chore: Remove dependency on io/ioutil package [[#355](https://github.com/ipfs-force-community/droplet/pull/355)]
* feat: persist shard to mysql [[#354](https://github.com/ipfs-force-community/droplet/pull/354)]
* feat: tools: generate car index and import index to mongo [[#356](https://github.com/ipfs-force-community/droplet/pull/356)]
* fix: close reader [[#363](https://github.com/ipfs-force-community/droplet/pull/363)]
* add issue template [[#361](https://github.com/ipfs-force-community/droplet/pull/361)]
* fix: import deals slow [[#371](https://github.com/ipfs-force-community/droplet/pull/371)]
* feat: add pprof [[#369](https://github.com/ipfs-force-community/droplet/pull/369)]
* Chore/merge v2.8 [[#372](https://github.com/ipfs-force-community/droplet/pull/372)]
* fix: memory leak [[#375](https://github.com/ipfs-force-community/droplet/pull/375)]
* fix: import boost deal [[#376](https://github.com/ipfs-force-community/droplet/pull/376)]
* doc:修正导入离线订单命令为./droplet storage deal import-data [[#380](https://github.com/ipfs-force-community/droplet/pull/380)]
* Create 模拟官方机器人HTTP方式检索 [[#385](https://github.com/ipfs-force-community/droplet/pull/385)]
* Fix/gen index tool [[#386](https://github.com/ipfs-force-community/droplet/pull/386)]
* chore: output average write to log [[#390](https://github.com/ipfs-force-community/droplet/pull/390)]
* update deploy docs / 更新部署文档 [[#392](https://github.com/ipfs-force-community/droplet/pull/392)]
* fix: not set retrieval deal status [[#391](https://github.com/ipfs-force-community/droplet/pull/391)]
* feat: add IAuthClient stub [[#399](https://github.com/ipfs-force-community/droplet/pull/399)]
* feat: support filter deals by SectorExpiration [[#404](https://github.com/ipfs-force-community/droplet/pull/404)]
* fix: handle slashed deal [[#402](https://github.com/ipfs-force-community/droplet/pull/402)]
* fix: Use the unified piecestorage object [[#409](https://github.com/ipfs-force-community/droplet/pull/409)]
* fix: parse address failed [[#411](https://github.com/ipfs-force-community/droplet/pull/411)]
* chore: update venus & go-data-transfer [[#397](https://github.com/ipfs-force-community/droplet/pull/397)]
* feat: Automatically delete temporary car files [[#413](https://github.com/ipfs-force-community/droplet/pull/413)]

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
