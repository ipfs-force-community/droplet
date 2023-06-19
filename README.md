<p align="center">
  <a href="https://damocles.venus-fil.io/" title="Damocles Docs">
    <img src="https://user-images.githubusercontent.com/1591330/205581638-e2955fa7-0da4-42fa-82d0-debd00b3f368.png" alt="Droplet Logo" width="128" />
  </a>
</p>

<h1 align="center">Droplet</h1>

<p align="center">
 <a href="https://github.com/ipfs-force-community/droplet/actions"><img src="https://github.com/ipfs-force-community/droplet/actions/workflows/build_upload.yml/badge.svg"/></a>
 <a href="https://codecov.io/gh/ipfs-force-community/droplet"><img src="https://codecov.io/gh/ipfs-force-community/droplet/branch/master/graph/badge.svg?token=J5QWYWkgHT"/></a>
 <a href="https://goreportcard.com/report/github.com/ipfs-force-community/droplet"><img src="https://goreportcard.com/badge/github.com/ipfs-force-community/droplet"/></a>
 <a href="https://github.com/ipfs-force-community/droplet/tags"><img src="https://img.shields.io/github/v/tag/ipfs-force-community/droplet"/></a>
  <br>
</p>

droplet will deliver a complete deal making experience as what lotus offers. This includes compatibility with lotus client where one can make deal with droplet using lotus client, retrieve deal/data in the same way as lotus retrieves its data, setup storage ask and etc.

Use [Droplet Issues](https://github.com/ipfs-force-community/droplet/issues) for reporting issues about this repository.

## feature
1. market 2.0 mainly implements the aggregation of multiple storage miners. clients can issue orders or retrieve any providers registered to droplet. 
2. all metadata of provider server is stored in the mysql database that providing better data security.
3. providers do not need to pay attention for the details of the deal, only need to query the market regularly to see if you have any deal to seal. 
4. market maintain a piece pool, that is, to provide the provider with the data for sealing deals, and it can also speed up the retrieval speed. ask miners for unseal operations, only when missing piece in droplet.
5. for clients, it is fully compatible with lotus.


## build

```sh
git clone https://github.com/ipfs-force-community/droplet.git
cd droplet
make
```
## how to set up droplet

run:

- run in chain service
```shell script
./droplet run --auth-url=<auth url> --node-url=<node url> --messager-url=<messager url>  --gateway-url=<signer url> --cs-token=<token of admin-authority> --signer-type="gateway"
```

- run in local, conn venus chain service and use lotus-wallet/venus-wallet to sign 
```shell script
./droplet run --auth-url=<auth url> --node-url=<node url> --messager-url=<messager url> --cs-token=<token of write-authority> --signer-type="wallet"  --signer-url=<wallet url> --signer-token=<wallet token>
```

- run in local, conn lotus full node and use lotus full node to sign
```shell script
./droplet run --node-url=<node url> --messager-url=<node url> --cs-token=<token of lotus> --signer-type="lotusnode"
```

set peer id and address

```shell script
./droplet net  listen                               #query droplet address and peerid
./droplet actor set-peer-id --miner <f0xxxx> <id>   #set peer id
./droplet actor set-addrs --miner <f0xxxx> <addr>   #set miner address
./droplet actor info --miner <f0xxxx>               #query miner address and peerid on chain
```

set storage ask
```shell script
./droplet storage-deals set-ask --price <price> --verified-price <price> --min-piece-size  <minsize >=256B>  --max-piece-size <max size <=sector-size> --miner <f0xxxx>
```

set retrieval ask
```shell script
./droplet retrieval-deals set-ask --price <price> --unseal-price <price> --payment-interval <bytes> --payment-interval-increase <bytes> --payment-addr <f0xxx>
```

## how to setup droplet client

```shell script
./droplet-client run --node-url <node url> --node-token <auth token>  --signer-url <wallet url> --signer-token  <wallet token> --addr <client default address>
```
Note:**please use a separate address, or maybe nonce conflict**
