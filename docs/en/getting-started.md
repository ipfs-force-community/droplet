# Getting Started

## Overview

`droplet` is the market service component in the `venus` system, which consists of two parts: `droplet` and `droplet-client`, commonly understood as market server and client.

- `droplet` serves storage providers;

- `droplet-client` serves users who have retrieval or storage demands.

The market service of `droplet` is divided into storage market and retrieval market, and its general process is as follows:



Storage process:

| Stages | Steps | Instructions |
|--------------------------------------|-------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------|
| Start `droplet` and `droplet-client` | 1. Configure and start [droplet](#Configure and Start droplet) <br> 2. Configure and start [droplet-client](#Configure and Start droplet-client) | |
| Proxy `libp2p` listener service for `miners` | via `droplet` [`libp2p` listener service for `miners`](#droplet proxy libp2p listener) | |
| Post storage ask for `miners` | via `droplet` [ask](#ask) | |
| Specify `miner` to propose deal | 1. Through `droplet-client` [import the data to be stored](#import the data to be stored) <br> 2. According to the demand [select the appropriate storage provider](#select the storage provider) < br> 3. [Initiate storage deal](#Initiate storage deal) | After the deal is issued, it takes a certain amount of time to execute the deal transaction process. After the deal is confirmed, the storage provider seal the data in the deal and submits the certificate. `droplet ` will be responsible for tracking order status. |

Retrieval process:

| Stages | Steps | Instructions |
| ---- | ---- | ---- |
| Start `droplet` and `droplet-client` | 1. Configure and start [droplet](#Configure and Start droplet) <br> 2. Configure and start [droplet-client](#Configure and Start droplet-client) | |
| Set retrieval price and payment address | Storage provider through `droplet` [set search pending order] (#retrieve pending order) | |
| Submit data retrieval request | [Submit data retrieval request] (#submit data retrieval request) | After submitting the data retrieval request, the search communication process will start, and the data will be returned in batches according to the agreement and fees will be transferred to the receiving address. |

:tipping_hand_woman: **Whether it is a storage deal or a retrieval request, the execution process of the protocol is automatic, and messages will be sent to the chain during the period, so it is necessary to ensure that the messages of both parties to the transaction can be signed normally. There is a necessary `fil` transfer in the transaction process, and the relevant address needs to have sufficient balance, otherwise the transaction will not be completed.**

## Configure and Start droplet

### Initialization

- On-chain mode

As a component of the chain service, it cooperates with `sophon-auth`, `venus`, `sophon-messager`, `sophon-gateway` and other components to provide market services for the `miner` registered to the chain service.

```
./droplet run \
--node-url=/ip4/<ip>/tcp/<port> \
--auth-url=http://<ip>:<port>\
--gateway-url=/ip4/<ip>/tcp/<port> \
--messager-url=/ip4/<ip>/tcp/<port> \
--cs-token=<shared-token> \
--signer-type="gateway"
```

Generated droplet service component configuration reference:
```toml
[Node]
   Url = "/ip4/192.168.200.151/tcp/3455"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"

[Messager]
   Url = "/ip4/127.0.0.1/tcp/39812"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"

[Signer]
   Type = "gateway"
   Url = "/ip4/127.0.0.1/tcp/45132"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"

[AuthNode]
   Url = "http://127.0.0.1:8989"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"
```

:tipping_hand_woman: **`shared-token` is used to authentication when accessing `API` of other chain service components. `token` is managed by `sophon-auth` and requires `admin` permission. For details, please refer to [sophon-auth token](https://sophon.venus-fil.io/intro/join-a-cs.html#for-admins-of-shared-modules)* *

- Off-chain mode

To start with `lotus fullnode`:

```
./droplet run \
--node-url=/ip4/<ip>/tcp/<port> \
--cs-token=<token of lotus> \
--signer-type="lotusnode"
```

Generated droplet service component configuration reference:
```toml
[Node]
   Url = "/ip4/127.0.0.1/tcp/1234"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.Ne3JsfHHhN6BgDtdsvLYfUaRC3eJPH_7KrBsMRBdplc"

[Messager]
   Url = ""
   Token = ""

[Signer]
   Type = "lotusnode"
   Url = "/ip4/127.0.0.1/tcp/1234"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.Ne3JsfHHhN6BgDtdsvLYfUaRC3eJPH_7KrBsMRBdplc"

[AuthNode]
   Url = ""
   Token = ""
```

When using chain service **and** `venus-wallet`:

```
./droplet run \
--auth-url=http://<ip>:<port>\
--node-url=/ip4/<ip>/tcp/<port> \
--messager-url=/ip4/<ip>/tcp/<port> \
--cs-token=<token of write-authority> \
--signer-url=/ip4/<ip>/tcp/<port> \
--signer-token=<token of venus-wallet> \
--signer-type="wallet"
```

Generated droplet service component configuration reference:
```toml
[Node]
   Url = "/ip4/192.168.200.151/tcp/3455"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"

[Messager]
   Url = "/ip4/127.0.0.1/tcp/39812"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"

[Signer]
   Type = "wallet"
   Url = "/ip4/127.0.0.1/tcp/5678/http"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIl19.IVBGlmRW__6g4QGbb1D1Jtr1oy
   MM6Sqs1tj1GDGR5EQ"

[AuthNode]
   Url = "http://127.0.0.1:8989"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoic2hhcmVkLXRva2VuIiwicGVybSI6ImFkbWluIiwiZXh0IjoiIn0.aARqJ_7FSe1KakkBhWlFvsYm-xBLAXBnl9SvTfqsVe8"
```

:tipping_hand_woman: **If signature uses an independent `venus-wallet` component, then configure it as the listening address of `venus-wallet` and a `token` with signing permission.**

To generate `venus-wallet` `token` with signing permission:

```bash
$ ./venus-wallet auth api-info --perm=sign
> eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIl19.Vr-OP7RNOozf6XXZuahpk-ZGA7IDa5NJjmU9L_znJ-0:/ip4/127.0/567/tcp
```

:tipping_hand_woman: **After the initialization command is successfully executed, `droplet repo` will be generated, and it needs to be configured according to the actual environment when using it. **

Configuration options will be generated when `droplet` starts for the first time. The default directory is: `~/.droplet/config.toml`. Next, we will introduce common configuration options.

### General Configuration

For the description of configuration items of `venus-wallet`, please refer to [droplet configuration](https://github.com/ipfs-force-community/droplet/blob/master/docs/zh/droplet%E9%85%8D%E7%BD%AE%E8%A7%A3%E9%87%8A.md), here we explain the more commonly used configuration items.

*tips:* After modifying the configuration file, you need to restart the `droplet` service:

```bash
$ nohup ./droplet run > droplet.log 2>&1 &
```
> After the `repo` has been generated, the parameters required for initialization are written into the configuration file, so there is no need to add them for subsequent startups.

#### Chain service configuration

- Including: chain synchronization node, message node, signature node and authentication node.

```yuml
[Node]
   Url = "/ip4/192.168.200.21/tcp/3453"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiemwiLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.3u-PInSUmX-8f6Z971M7JBCHYgFVQrvwUjJfFY03ouQ"
[Messager]
   Url = "/ip4/192.168.200.21/tcp/39812"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiemwiLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.3u-PInSUmX-8f6Z971M7JBCHYgFVQrvwUjJfFY03ouQ"
[Signer]
   Type = "gateway"
   Url = "/ip4/192.168.200.21/tcp/45132"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiemwiLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.3u-PInSUmX-8f6Z971M7JBCHYgFVQrvwUjJfFY03ouQ"
[AuthNode]
   Url = "http://192.168.200.21:8989"
   Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiemwiLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.3u-PInSUmX-8f6Z971M7JBCHYgFVQrvwUjJfFY03ouQ"
```

#### `API` listener configuration

The default listening port of `droplet` is `127.0.0.1:41235`, in order to support access requests from different networks, the listening address of `API` needs to be modified:

```yuml
[API]
ListenAddress = "/ip4/0.0.0.0/tcp/41235"
```
 
#### `PublishMsgPeriod` configuration

When `droplet` receives a deal from `droplet-client`, it will not publish the `ClientDealProposal` message immediately, but will wait for a certain period of time, which is controlled by the `PublishMsgPeriod` option in the configuration file, which can be used during testing Setting this to a small value reduces waiting time. The following settings, set the wait time to 10 seconds.

```yuml
PublishMsgPeriod = "10s"
```

#### `PieceStorage` Configuration

Currently `droplet` supports two storage modes for `Piece` data:
- File system
- Object storage

```yuml
[Piece Storage]
   [[PieceStorage. Fs]]
     Name = "local"
     Enable = true
     Path = "/mnt/pieces"
   [[PieceStorage.S3]]
     Name = "oss"
     Enable = false
     EndPoint = ""
     AccessKey = ""
     SecretKey = ""
     Token = ""
```

It can also be configured by command:

```bash
# Local file system storage
./droplet piece-storage add-fs --path="/piece/storage/path" --name="local"

# object storage
./droplet piece-storage add-s3 --endpoint=<url> --name="oss"
```

#### `Miners` Configuration

The miners of the `droplet` service and the parameters of each miner are configured as follows:

```
[[Miners]]
   Addr = "f01000"
   Account = "testuser01"
  
   ConsiderOnlineStorageDeals = true
   Consider OfflineStorageDeals = true
   ConsiderOnlineRetrievalDeals = true
   ConsiderOfflineRetrievalDeals = true
   ConsiderVerifiedStorageDeals = true
   ConsiderUnverifiedStorageDeals = true
   PieceCidBlocklist = []
   ExpectedSealDuration = "24h0m0s"
   MaxDealStartDelay = "336h0m0s"
   PublishMsgPeriod = "1h0m0s"
   MaxDealsPerPublishMsg = 8
   MaxProviderCollateralMultiplier = 2
   Filter = ""
   RetrievalFilter = ""
   TransferPath = ""
   MaxPublishDealsFee = "0 FIL"
   MaxMarketBalanceAddFee = "0 FIL"
   [CommonProviderConfig. RetrievalPricing]
     Strategy = "default"
     [CommonProviderConfig. RetrievalPricing. Default]
       VerifiedDealsFreeTransfer = true
     [CommonProviderConfig. RetrievalPricing. External]
       Path = ""
     [CommonProviderConfig. AddressConfig]
       DisableWorkerFallback = false
```

:::tip

If there are multiple miners, just copy the above configuration. *** If there are many miners, the configuration file will be very long, consider optimizing? ***

:::

## droplet proxy libp2p listener

Setting `droplet` as `miner`'s `libp2p` listener proxy is to set the entrance of a specific `miner` market service to the current running `droplet` instance.

- Query `droplet` peer node listening address

```bash
./droplet net listen

/ip4/127.0.0.1/tcp/58418/p2p/12D3KooWQftXTGFBKooKuyaNkugapUzi4VmjxEKTgkpsNCQufKBK
/ip4/192.168.19.67/tcp/58418/p2p/12D3KooWQftXTGFBKooKuyaNkugapUzi4VmjxEKTgkpsNCQufKBK
/ip6/::1/tcp/49770/p2p/12D3KooWQftXTGFBKooKuyaNkugapUzi4VmjxEKTgkpsNCQufKBK
```

- set `Mutiadrs` and `peerid` of `miners` to `Mutiadrs` and `peerid` of `droplet`

```bash
./droplet actor set-addrs --miner=t01041 /ip4/192.168.19.67/tcp/58418
Requested multiaddrs change in message bafy2bzaceceqgxmiledunzjwbajpghzzn4iibvxhoifsrz4q2grzsirgznzdg

./droplet actor set-peer-id --miner=f01041 12D3KooWQftXTGFBKooKuyaNkugapUzi4VmjxEKTgkpsNCQufKBK
   Requested peerid change in message bafy2bzacea4ruzf4hvyezzhjkt6hnzz5tpk7ttuw6jmyoadqasqtujypqitp2
```

- After waiting for the message to be on-chained, check the agent information of `miner`

```bash
./droplet actor info --miner t01041
peers: 12D3KooWQftXTGFBKooKuyaNkugapUzi4VmjxEKTgkpsNCQufKBK
addr: /ip4/192.168.19.67/tcp/58418
```

## Storage Ask

### Storage Deal Ask

```bash
./droplet storage ask set --price=0.01fil --verified-price=0.02fil --min-piece-size=512b --max-piece-size=512M t01041
```
Pricing information can be checked via the command line tool:

```shell
./droplet storage ask get t01041
Price per GiB/Epoch Verified Min. Piece Size (padded) Max. Piece Size (padded) Expiry (Epoch) Expiry (Appx. Rem. Time) Seq. No.
0.01 FIL 0.02 FIL 512 B 521 MiB 161256 719h59m0s 0
```

### Retrieve Deal

The storage provider should at least set the payment address

```bash
./droplet retrieve ask set t3ueb62v5kbyuvwo5tuyzpvds2bfakdjeg2s33p47buvbfiyd7w5fwmeilobt5cqzi673s5z6i267igkgxum6a
```

At the same time, you can also set the price of the data retrieval request, if not set, the default is 0.
```bash
./droplet retrieve ask set \
--price 0.02fil \
--unseal-price 0.01fil \
--payment-interval 1MB \
t3ueb62v5kbyuvwo5tuyzpvds2bfakdjeg2s33p47buvbfiyd7w5fwmeilobt5cqzi673s5z6i267igkgxum6a
```


## Configure and Start droplet-client

The normal operation of `droplet-client` requires synchronization node, signature node (`venus fullnode` and `lotus fullnode` can be used as signature nodes), message nodes (`venus fullnode` and `lotus fullnode` can be used as message nodes) and `droplet`, which means it can be configured flexibly, as long as the message can be signed properly and snet to the chain.

`droplet-client` needs to be configured with `--addr` to bind the client’s wallet address, which is used to pay client collateral and retrieval fees.

Here are three commonly used startup methods:

- Access to the `Venus` chain service

The signature `API` of `sophon-gateway` can only be accessed with `admin` permission (for security considerations). It is not recommended to use `sophon-gateway` for `droplet-client`. We use the local `venus-wallet` for sign in this case.

```shell
./droplet-client run \
--node-url=/ip4/<venus_ip>/tcp/<port> \
--messager-url=/ip4/<sophon-messager_ip>/tcp/<port> \
--auth-token=<user-signed-token> \
--signer-type=wallet \
--signer-url=/ip4/<venus-wallet_ip>/tcp/<port> \
--signer-toke=<wallet-token> \
--addr=<signer address> \
```

> `venus-wallet` generates `token` with signature permission. please refer to the above.


- Connect to `lotus fullnode` and start
```shell
./droplet-client run \
--node-url=/ip4/<venus_ip>/tcp/<port> \
--node-token=<node-token> \
--signer-type=lotusnode\
--addr=<signer address> \
```

- Connect to `venus fullnode` and start
```shell
./droplet-client run \
--node-url=/ip4/<venus_ip>/tcp/<port> \
--node-token=<node-token> \
--signer-type=wallet \
--addr=<signer address> \
```

These configuration items can also be set in configuration files, see [droplet-client configuration](https://github.com/ipfs-force-community/droplet/blob/master/docs/zh/droplet-client%E9%85%8D%E7%BD%AE%E8%A7%A3%E9%87%8A.md)


## Storage Deal

### Import the data to be stored

```shell
./droplet-client data import <file path>
Import 1642491708527303001, Root bafk2bzacedgv2xqys5ja4gycqipmg543ekxz3tjj6wwfexda352n55ahjldja
```

### Choose Storage Provider

Use `droplet-client` to query `miner` storage ask information:

```bash
./droplet-client storage asks query f01041
Ask: t01041
Price per GiB: 0.02 FIL
Verified Price per GiB: 0.01 FIL
Max Piece size: 8 MiB
Min Piece size: 512 B
```

### Initiate Storage Deal

```shell
/droplet-client storage deals init
# Enter the cid of the data to be stored, `./droplet-client data local`command to view
Data CID (from lotus client import): bafk2bzacedgv2xqys5ja4gycqipmg543ekxz3tjj6wwfexda352n55ahjldja
.. calculating data size
PieceCid: baga6ea4seaqpz47j4kqdiixpehmzk3uw5c4cmqvs5iyi7xf7rwkepfhdvowdiai PayLoadSize: 809 PieceSize: 1024
# Enter the data storage period
Deal duration (days): 180
Miner Addresses (f0.. f0..), none to find: t01041
.. querying miner asks
-----
Proposing from t16qnfduxzpneb2m3rbdasvhgk7rmmo32zpiypkaq
Balance: 9499.999999856612207905 FIL
Piece size: 1KiB (Payload size: 809B)
Duration: 4320h0m0s
Total price: ~0.0098876953124352 FIL (0.000000019073486328 FIL per epoch)
Verified: false
# Confirm whether to accept the order price
Accept (yes/no): yes
.. executing
Deal (t01051) CID: bafyreihiln2ha6eaaos7kuhwpnvjvjlxmjnpsklym6hhucu2z776bf2or4
```

Then wait for the dael message to be sent to the chain and the storage provider to complete the data sealing.

`droplet-client` view daels:
```shell
./droplet-client storage deals list
DealCid DealId Provider State On Chain? Slashed? PieceCID Size Price Duration Verified
...76bf2or4 0 t01051 StorageDealCheckForAcceptance N N ...dvowdiai 1016 B 0.00992212295525724 FIL 520205 false
   Message: Provider state: StorageDealPublish
```

### Offline Storage Deal

1. Import storage deal file

```bash
./droplet-client data import ./README.md
Import 1642643014364955003, Root bafk2bzaceaf4sallirkt63fqrojz5gaut7akiwxrclcsymqelqad7man3hc2c
```

2. Convert to CAR file

```bash
./droplet-client data generate-car ./README.md ./readme.md.car
```

3. Calculate the `CID` and `Piece size` of the CAR file

```shell
./droplet-client data commP ./readme.md.car
CID: baga6ea4seaqfewgysi3n3cjylkbfknr56vbemb2gwjfvpctqtjgpdweu7o3d6mq
Piece size: 3.969 KiB
```

4. Initiate a deal

```bash
./droplet-client storage deals init \
--manual-piece-cid=baga6ea4seaqfewgysi3n3cjylkbfknr56vbemb2gwjfvpctqtjgpdweu7o3d6mq \
--manual-piece-size=4064 \
bafk2bzaceaf4sallirkt63fqrojz5gaut7akiwxrclcsymqelqad7man3hc2c \
f01051\
0.01fil \
518400
bafyreiecguaxgtmgcanfco6huni4d6h6zs3w3bznermmiurtdos7r6hftm
```

- `manual-piece-cid`: `CID` output after executing `data commP` in step 3
- `manual-piece-size`: `Piece size` output after executing `data commP` in step 3. It should be noted that when using this parameter, this value needs to be converted into the size of `byte`, here The size converted to byte for 3.969kib is 4064.
The next four parameters are:
- The `Root` entered after executing the `import` command in the first step
- miner ID
- Negotiate to pay `0.01fil` for the order, **this value must be greater than the minimum value in `storage ask` set by miner, otherwise the request will be rejected.
- Contract period, must be greater than or equal to 180 days, this value also needs to be replaced by epoch, each epoch=30 seconds, in the example: 518400 = 180 days.

The final output `bafyreidfs2w7lxacq6zpqck7q4zimyitidxyahojf7dbbuz5zr7irdlmqa` is the proposed CID.
Like online deals, you can check the deal information through the droplet-client at this time, and the final status of the order will stop at `StorageDealWaitingForData`

```shell
./droplet-client storage deals list
DealCid DealId Provider State On Chain? Slashed? PieceCID Size Price Duration Verified
...s7r6hftm0 t01051 StorageDealCheckForAcceptance N N ... u7o3d6mq 3.969 KiB 5196.63 FIL 519663 false
   Message: Provider state: StorageDealWaitingForData
```

:tipping_hand_woman: **If `droplet-client` shows the following content:**
```shell
2022-01-20T12:47:27.966+0800  ERROR storagemarket_impl   clientstates/client_states.go:324   deal bafyreif2k2e4acraxk33z3llwo5gqmk32tfrdj2kocjanojbfbf6vj72vm failed: adding market funds failed: estimating gas used: message execution failed: exit SysErrInsufficientFunds(6)
```
It means that the balance in the wallet is insufficient, call the command `./droplet-client actor-funds add 100fil` and re-execute the command.

5. Import data files of offline deal
It is necessary to transfer the `.car` file generated in the previous step 2 to droplet offline, and import the data through the droplet command:
```shell
./droplet storage-deals import-data bafyreiecguaxgtmgcanfco6huni4d6h6zs3w3bznermmiurtdos7r6hftm ./readme.md.car
```

Check the status again, the order status is updated to `StorageDealPublishing`:
```shell
./droplet-client storage deals list
DealCid DealId Provider State On Chain? Slashed? PieceCID Size Price Duration Verified
...s7r6hftm 0 t01051 StorageDealCheckForAcceptance N N ...u7o3d6mq 3.969 KiB 5196.63 FIL 519663 false
Message: Provider state: StorageDealPublishing
```

Finally, wait for the deal status to change to `StorageDealAwaitingPreCommit`, then the deal data is ready to be sealed.


## Submit Data Retrieval Request

Users can initiate data retrieval request by `minerID` and `Data CID`

```shell
./droplet-client retrieval retrieve --provider t01020 bafk2bzacearla6en6crpouxo72d5lhr3buajbzjippl63bfsd2m7rsyughu42 test.txt
Recv 0 B, Paid 0 FIL, Open (New), 0s
Recv 0 B, Paid 0 FIL, DealProposed (WaitForAcceptance), 16ms
Recv 0 B, Paid 0 FIL, DealAccepted (Accepted), 26ms
Recv 0 B, Paid 0 FIL, PaymentChannelSkip (Ongoing), 27ms
Recv 1.479 KiB, Paid 0 FIL, Blocks Received (Ongoing), 30ms
Recv 1.479 KiB, Paid 0 FIL, AllBlocksReceived (BlocksComplete), 33ms
Recv 1.479 KiB, Paid 0 FIL, Complete (CheckComplete), 35ms
Recv 1.479 KiB, Paid 0 FIL, CompleteVerified (Finalizing Blockstore), 36ms
Recv 1.479 KiB, Paid 0 FIL, BlockstoreFinalized (Completed), 36ms
Success
```