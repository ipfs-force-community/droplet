# venus-market
market for storage and retrieval

venus-market组件实现中分为两个大的目标：
1. 实现单矿工版本， 和lotus客户端完全兼容，能够配合venus矿工完成数据的存储和检索功能
2. 基于venus分布式矿池的概念，完成一个集中样式的market中心，解决矿工在接单，检索方面遇到的问题。

实现的主要功能：
1. 兼容lotus的数据存储，实现能够通过lotus向venus-market进行数据存储过程。
2. 兼容lotus的数据检索，实现能够通过lotus向venus-marke他进行数据检索的过程。
3. 缩减接口，把重要的数据全部保存在venus-market本地数据。
4. 多样化数据传输，支持除了Graphsync之外其他的传输模式。
5. 数据高可用，支持主要元数据存储到数据库里面。
6. 支持多租户的market市场，可以作为venus矿池的订单出入的入口，整合下属所有矿工的存储检索能力。
7. 撮合用户订单和矿工，实现piece数据的自动多订单备份，及检索的多点多地区备份
8. venus-market作为ipfs网关接入到ipfs网络，探索收费ipfs节点的使用方式。
6. 轻量级市场客户端，在支持传统模式的同时也支持venus-market特色的功能特性。



