# venus 链服务组件间签名机制

## 背景介绍

`venus` 的 `account` 体系中，各角色的关系如下：

- `venus-auth` 为指定的用户（`account`）生成所需权限的 `token`， `SP` 用户用此 `token` 访问链服务接口；
- `venus-auth` 保存 `account-minerIDs`，即一个用户可以拥有对个 `minerID`；
- 链服务组件之间的接口访问没有用到用户的 `token`, 是系统内置的共享 `token`

### 用户请求的签名机制

以 `venus-cluster` 推送消息的过程为例：

- 以 `token` 请求 `venus-messager` 的接口:

```
PushMessage(ctx context.Context, msg *shared.Message, meta *types.SendSpec) (string, error)
```

- `venus-messager` 处理 `rpcapi` 请求，从 `token` 中解析获得在 `venus-auth` 中生成的 `account`, 然后调用 `venus-gateway` 中的签名接口:

```
WalletSign(ctx context.Context, account string, addr address.Address, toSign []byte, meta venusTypes.MsgMeta) (*crypto.Signature, error)
```

- `venus-gateway` 根据 `account` 找到对应的 `venus-wallet` 服务进行签名。 

### 链服务内部的签名机制

 链服务组件之间的接口访问用的是内置的 `share token`，故无法通过 `token` 解析出 `account`， 这样就需要依赖 `venus-auth` 的 保存 `account-minerIDs` 体系。以 `venus-miner` 请求 `venus-gateway` 为例进行说明。
 
- `venus-miner` 从 `venus-auth` 获取 `account-miners` 列表；
- 先从`account-miners` 列表中查找 `minerID` 对应的 `account`， 然后调用 `venus-gateway` 的 `WalletSign` 接口。


## 链服务账号管理

### 问题分析

本问题以接入 `venus` 链服务的场景进行分析

- 链服务中没有 `signer address` 与 `account` 的关系表，而依赖链服务的签名接口需要 `account` 参数，`venus-market` 需要额外扩展 `signer address-account` 关系，如下：

假设 `f0128788` 的可签名地址有: `f3wylwd6pclppme4qmbgwled5xpsbgwgqbn2alxa7yahg2gnbfkipsdv6m764xm5coizujmwdmkxeugplmorha`, `f3r47fkdzfmtex5ic3jnwlzc7bkpbj7s4d6limyt4f57t3cuqq5nuvhvwv2cu2a6iga2s64vjqcxjqiezyjooq`。

初始 `miner-account`, `venus-auth` 中实现的关系：
```
f0128788-test
```

扩展后，`venus-market` 中的扩展: 
```
f0128788-test
f3wylwd6pclppme4qmbgwled5xpsbgwgqbn2alxa7yahg2gnbfkipsdv6m764xm5coizujmwdmkxeugplmorha-test
f3r47fkdzfmtex5ic3jnwlzc7bkpbj7s4d6limyt4f57t3cuqq5nuvhvwv2cu2a6iga2s64vjqcxjqiezyjooq-test
```

- 存储市场跟踪订单或扇区状态产生了不必要的工作量，会把非 `miner Actor` 的地址也进行遍历，如下：
```
func (dealTracker *DealTracker) scanDeal(ctx metrics.MetricsCtx) {
	addrs, err := dealTracker.minerMgr.ActorAddress(ctx)
	if err != nil {
		log.Errorf("get miners list %w", err)
	}
	head, err := dealTracker.fullNode.ChainHead(ctx)
	if err != nil {
		log.Errorf("get chain head %w", err)
	}

	for _, addr := range addrs {
		dealTracker.checkSlash(ctx, addr, head.Key())
		dealTracker.checkPreCommitAndCommit(ctx, addr, head.Key())
	}
}
```

### 改进方案

- `venus-auth` 建立 `account` 与私钥（signer address）映射

- `venus-auth` 解除 `minerID` 与 `account` 映射，建立 `minerID` 与 `signer address` 的映射

- `venus-gateway` 验证注册的 `signer address` 服务,  抢占式写入到 `venus-auth`；


存在的疑问： `venus-auth` 是否需要保存 `minerID` 表：

- 如果不保存，`venus-miner` 和 `venus-gateway` 该如何知晓为那些矿工服务；

- 这时就需要从 `venus-gateway` 获取可提供服务的矿工列表，并且需要轮询列表的变化；

- `venus-gateway` 的矿工来源于 `venus-cluster` 的注册，也就是说只有矿工的 `venus-cluster` 注册到 `venus-gateway` 后才能提供市场和出块服务；

  - `venus-miner` 的服务依赖于 `venus-cluster` 正常运行（能正确执行 `ComputeProof`）, 故从`venus-gateway` 获取矿工列表执行出块没有问题；

  - `venus-market` 的接发单服务是不依赖于 `venus-cluster` 的，设计上矿工的 `venus-cluster` 没有注册到 `venus-gateway`也不妨碍其接发单的。这种情况下 `venus-market` 为那些矿工提供市场服务就没有了来源。
  
综上所述，venus链服务的设计初衷是先要知道为那些矿工提供服务的，因此 account-miner 关系仍然保持。



## 内外部接口协调

`venus-market` 的实现是整合了其作为客户端和服务端的不同逻辑，为了匹配更多的不同实现，如 `market-client` 用 `lotus fullnode` 或 `venus fullnode` 推送消息，用 `lotus wallet` 或 `venus wallet` 进行签名等，由于不同的实现在接口定义上存在差异，故需要整合不同的实现方式。 

### 消息接口

`venus-market` 对外接口不变，移除 `venus-messager` 中依赖 `account` 的接口，统一内外部接口：

```
type IMessager interface {
    PushMessage(ctx context.Context, msg *types.Message, meta *mtypes.SendSpec) (string, error)                                                                  //perm:write
    PushMessageWithId(ctx context.Context, id string, msg *types.Message, meta *mtypes.SendSpec) (string, error)                                                 //perm:write
}
```

`venus-messager` 中移除根据 `token` 解析 `account` 的逻辑，由 `venus-gateway` 处理,详见 `venus-gateway` 的设计文档。 


### 签名接口

目前的签名方式有三种：`venus-wallet/lotus-wallet/venus`， 接口如下:

```
type ISigner interface {
	WalletHas(ctx context.Context, signerAddr address.Address) (bool, error)
	WalletSign(ctx context.Context, signerAddr address.Address, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error)
}
```

而 `lotus fullnode` 的签名接口定义：

```
// WalletHas indicates whether the given address is in the wallet.
WalletHas(context.Context, address.Address) (bool, error) //perm:write
// WalletSign signs the given bytes using the given address.
WalletSign(context.Context, address.Address, []byte) (*crypto.Signature, error) //perm:sign
```

支持 `lotus fullnode` 进行签名，并对其进行降级：

```
type LotusnodeClient struct {
	Internal struct {
		WalletHas  func(context.Context, address.Address) (bool, error)
		WalletSign func(context.Context, address.Address, []byte) (*vCrypto.Signature, error)
	}
}

func (lnw *LotusnodeClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return lnw.Internal.WalletHas(ctx, addr)
}

func (lnw *LotusnodeClient) WalletSign(ctx context.Context, addr address.Address, msg []byte) (*vCrypto.Signature, error) {
	return lnw.Internal.WalletSign(ctx, addr, msg)
}

type WrapperLotusnodeClient struct {
	lotusnodeClient *LotusnodeClient
}

func (w *WrapperLotusnodeClient) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return w.lotusnodeClient.WalletHas(ctx, addr)
}

func (w *WrapperLotusnodeClient) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta vTypes.MsgMeta) (*vCrypto.Signature, error) {
	return w.lotusnodeClient.WalletSign(ctx, addr, msg)
}

func newLotusnodeClient(ctx context.Context, nodeCfg *config.Signer) (ISigner, jsonrpc.ClientCloser, error) {
	apiInfo := apiinfo.NewAPIInfo(nodeCfg.Url, nodeCfg.Token)
	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, err
	}

	lotusnodeClient := LotusnodeClient{}
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin", []interface{}{&lotusnodeClient.Internal}, apiInfo.AuthHeader())

	return &WrapperLotusnodeClient{lotusnodeClient: &lotusnodeClient}, closer, err
}
```

- `market-client` 没有实现通过 `venus` 的链服务发单（没法签名）, 需要实现。

- `market-client` 和 `venus-market` 都实现 `ISigner`，移除对于 `fullnode` 接口的注入转换：

```
switch signerCfg.SignerType {
// Sign with lotus node
case config.SignerTypeLotusnode:
	signer, closer, err = newLotusnodeClient(mCtx, signerCfg)
// Sign with lotus-wallet/venus-wallet/venus fullnode
case config.SignerTypeWallet:
	signer, closer, err = newWalletClient(mCtx, signerCfg)
// Signing through venus chain-service
case config.SignerTypeGateway:
	if !isServer {
		return nil, fmt.Errorf("signing through the venus-gateway cannot be used for market-client")
	}
	signer, closer, err = newGatewayWalletClient(mCtx, signerCfg)
default:
	return nil, fmt.Errorf("unsupport signer type %s", signerCfg.SignerType)
}
```
