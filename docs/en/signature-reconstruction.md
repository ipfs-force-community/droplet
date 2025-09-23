# Venus Chain Service Component Signing Mechanism

## Background Introduction

In the `venus` `account` system, the relationships between the various roles are as follows:

- `sophon-auth` generates `tokens` with required permissions for specified users (`accounts`). `SP` users use this `token` to access chain service interfaces.
- `sophon-auth` stores `account-minerIDs`, meaning one user can own multiple `minerIDs`.
- Interface access between chain service components does not use the user's `token`; it uses a built-in shared `token`.

### User Request Signing Mechanism

Taking the message push process of `sophon-cluster` as an example:

- Request `sophon-messager`'s interface with a `token`:
```
PushMessage(ctx context.Context, msg shared.Message, meta types.SendSpec) (string, error)
```

- `sophon-messager` processes the `rpcapi` request, parses the `account` generated in `sophon-auth` from the `token`, and then calls the signing interface in `sophon-gateway`:
```
  WalletSign(ctx context.Context, account string, addr address.Address, toSign []byte, meta venusTypes.MsgMeta) (*crypto.Signature, error)
```
- `sophon-gateway` finds the corresponding `venus-wallet` service based on the `account` to perform the signing.

### Internal Signing Mechanism of Chain Services

Interface access between chain service components uses the built-in `share token`, so the `account` cannot be parsed from the `token`. This creates a dependency on the `account-minerIDs` system stored by `sophon-auth`. Take `venus-miner` requesting `sophon-gateway` as an example for explanation.

- `venus-miner` obtains the `account-miners` list from `sophon-auth`.
- It first looks up the `account` corresponding to the `minerID` from the `account-miners` list, and then calls the `WalletSign` interface of `sophon-gateway`.

### Problem Analysis

This issue is analyzed based on the scenario of integrating with the `venus` chain service.

- The chain service lacks a relationship table between `signer address` and `account`. However, the chain service's signing interface requires an `account` parameter. Therefore, `droplet` needs to additionally extend the `signer-account` relationship, as shown below:

Assuming the signable addresses for `f0128788` are: `f3wylwd6pclppme4qmbgwled5xpsbgwgqbn2alxa7yahg2gnbfkipsdv6m764xm5coizujmwdmkxeugplmorha`, `f3r47fkdzfmtex5ic3jnwlzc7bkpbj7s4d6limyt4f57t3cuqq5nuvhvwv2cu2a6iga2s64vjqcxjqiezyjooq`.

Initial `miner-account` relationship, implemented in `sophon-auth`:

```f0128788-test```

After extension, the extension in `droplet`:

f0128788-test
f3wylwd6pclppme4qmbgwled5xpsbgwgqbn2alxa7yahg2gnbfkipsdv6m764xm5coizujmwdmkxeugplmorha-test
f3r47fkdzfmtex5ic3jnwlzc7bkpbj7s4d6limyt4f57t3cuqq5nuvhvwv2cu2a6iga2s64vjqcxjqiezyjooq-test


- Tracking deal or sector status in the storage market generates unnecessary workload because it also traverses addresses that are not `miner Actor` addresses, as shown below:
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

## Specification (Feature Spec)

This refactoring involves components including: sophon-auth, sophon-gateway, sophon-messager, venus-miner, droplet, etc. The modifications for each will be introduced below.

### sophon-auth

`sophon-auth` provides interfaces to bind signing addresses to accounts and offers query interfaces, ensuring the chain service can securely support multi-user mutual signing. The interfaces are as follows:

```
type IAuthClient interface {
    GetUserBySigner(signer string) (auth.ListUsersResponse, error)
    RegisterSigners(user string, addrs []string) error
    UnregisterSigners(user string, addrs []string) error
}
```

### sophon-gateway

Signature interface refactoring:

```
type IWalletClient interface {
    WalletHas(ctx context.Context, addr address.Address, accounts []string) (bool, error)                                                  //perm:admin
    WalletSign(ctx context.Context, addr address.Address, accounts []string, toSign []byte, meta types.MsgMeta) (*crypto.Signature, error) //perm:admin
}
```

- The signing interface of `sophon-gateway` is only accessible to `tokens` with `admin` permissions, meaning it can only be accessed internally by chain services, such as: `venus-miner`, `sophon-messager`.
- Permission checks must be completed before calling the signing interface of `sophon-gateway`. This is to prevent constructing messages and calling the private keys under someone else's account for signing.
- When calling the signing interface of `sophon-gateway`, the account bound to the key must be provided. This is to allow `venus-wallet` instances of multiple accounts to sign for each other.

Automatic registration/deregistration of signing addresses:

- When a `venus-wallet` establishes a connection, validate the private keys it manages and the supported accounts it has configured. After validation, call the interface to save the information to `sophon-auth`. For example, if a `venus-wallet` terminal manages private keys: w1, w2; and configured supported accounts: user01, user02; then the bound relationships would be: user01-w1, user01-w2, user02-w1, user02-w2.
- Automatically deregister the binding relationship between the signing address and the account when the `venus-wallet` connection is disconnected or when an address is deleted through an interface.

### sophon-messager

Remove the forced message push interfaces. SP will use a unified message push interface.

```
type IMessager interface {
  ForcePushMessage(ctx context.Context, account string, msg types.Message, meta mtypes.SendSpec) (string, error)                                             //perm:admin

  ForcePushMessageWithId(ctx context.Context, id string, account string, msg types.Message, meta mtypes.SendSpec) (string, error)                            //perm:write
}
```

Message push process modification:

- When SP components call chain services, they usually use a `token` with [`sign`, `read`, `write`] permissions. `sophon-messager` must first check if the permissions of the `token` carried with the interface call are sufficient:
  - The account corresponding to the `token` is bound to the signing address `msg.From` in the message.
  - The `token` has `sign` permission (to be confirmed).
- Provide all accounts bound to the signing address for the signing interface. This way, as long as one signing terminal (`venus-wallet`) connected to `sophon-gateway` supports signing for this key, this message can be signed, enabling mutual signing across multiple accounts.

### venus-miner

No logical changes, only needs to adapt to the new interface calls.

### droplet

- Remove the logic for additionally extending the `signer-account` relationship.
- Before signing a message, check the permissions of the request `token`. After passing the check, call the signing interface of `sophon-gateway`.

Note regarding the `droplet-client` component: although it is implemented within the same project as `droplet`, its nature is that of an SP component. Its signing process cannot go through `sophon-gateway`. Its signing logic is:

- Signing for deals requires an additional signing terminal (`venus-wallet` or `lotus/venus`).
- Messages during the transaction process are pushed to `sophon-messager`, consistent with how messages are pushed by `sophon-cluster`, etc.

> Therefore, would it be better to separate the `droplet-client` component code into an independent project?
