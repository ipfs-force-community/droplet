<h1 align="center">Venus Market</h1>

<p align="center">
 <a href="https://github.com/filecoin-project/venus-market/actions"><img src="https://github.com/filecoin-project/venus-market/actions/workflows/build_upload.yml/badge.svg"/></a>
 <a href="https://codecov.io/gh/filecoin-project/venus-market"><img src="https://codecov.io/gh/filecoin-project/venus-market/branch/master/graph/badge.svg?token=J5QWYWkgHT"/></a>
 <a href="https://goreportcard.com/report/github.com/filecoin-project/venus-market"><img src="https://goreportcard.com/badge/github.com/filecoin-project/venus-market"/></a>
 <a href="https://github.com/filecoin-project/venus-market/tags"><img src="https://img.shields.io/github/v/tag/filecoin-project/venus-market"/></a>
  <br>
</p>

Use [Venus Issues](https://github.com/filecoin-project/venus/issues) for reporting issues about this repository.

venus-market will deliver a complete deal making experience as what lotus offers. This includes compatibility with lotus client where one can make deal with venus-market using lotus client, retrieve deal/data in the same way as lotus retrieves its data, setup storage ask and etc.

Use [Venus Issues](https://github.com/filecoin-project/venus/issues) for reporting issues about this repository.

## feature
1. market 2.0 mainly implements the aggregation of multiple storage miners. clients can issue orders or retrieve any providers registered to venus-market. 
2. all metadata of provider server is stored in the mysql database that providing better data security.
3. providers do not need to pay attention for the details of the deal,  only need to query the market regularly to see if you have any deal to seal. 
4. market maintain a piece pool, that is, to provide the provider with the data for sealing deals, and it can also speed up the retrieval speed. ask miners for unseal operations, only when missing piece in venus-market.
5. for clients, it is fully compatible with lotus.


## build

```sh
git clone https://github.com/filecoin-project/venus-market.git
cd venus-market
make
```
## how to set up venus-market

run:

- run in chain service
```shell script
./venus-market run --auth-url=<auth url> --node-url=<node url> --messager-url=<messager url>  --gateway-url=<signer url> --cs-token=<token of admin-authority> --signer-type="gateway"
```

- run in local, conn venus chain service and use lotus-wallet/venus-wallet to sign 
```shell script
./venus-market run --auth-url=<auth url> --node-url=<node url> --messager-url=<messager url> --cs-token=<token of write-authority> --signer-type="wallet"  --signer-url=<wallet url> --signer-token=<wallet token>
```

- run in local, conn lotus full node and use lotus full node to sign
```shell script
./venus-market run --node-url=<node url> --messager-url=<node url> --cs-token=<token of lotus> --signer-type="lotusnode"
```

set peer id and address

```shell script
./venus-market net  listen                               #query venus-market address and peerid
./venus-market actor set-peer-id --miner <f0xxxx> <id>   #set peer id
./venus-market actor set-addrs --miner <f0xxxx> <addr>   #set miner address
./venus-market actor info --miner <f0xxxx>               #query miner address and peerid on chain
```

set storage ask
```shell script
./venus-market storage-deals set-ask --price <price> --verified-price <price> --min-piece-size  <minsize >=256B>  --max-piece-size <max size <=sector-size> --miner <f0xxxx>
```

set retrieval ask
```shell script
./venus-market retrieval-deals set-ask --price <pirce> --unseal-price <price> --payment-interval <bytes> --payment-interval-increase <bytes> --payment-addr <fxxx>
```

## how to setup market client

```shell script
./market-client run --node-url <node url> --node-token <auth token>  --signer-url <wallet url> --signer-token  <wallet token> --addr <client default address>
```
Note:**please use a seperate address, or maybe nonce confiction**
