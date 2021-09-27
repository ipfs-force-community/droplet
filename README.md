# venus-market

venus-market will deliver a complete deal making experience as what lotus offers. This includes compatibility with lotus client where one can make deal with venus-market using lotus client, retrieve deal/data in the same way as lotus retrieves its data, setup storage ask and etc.

* Implementation of the one-to-one model of lotus market like module and fully interoperable with lotus implementation, which means compatibility with lotus client and more
* venus-market deployed as independent module, like venus-sealer and venus-wallet
* Implementation of a reliable market module that runs a seperate process from the main storage process
* A clear module boundary that allows interoperability and user customizations
* Flexibilities of market module to interact with existing venus infrastructures using RPC APIs
* Supports for mainnet, calibration and Nerpa
* Lightweight client: compatibility with Lotus and support for venus-market unique features including client running seperately as a process and remove dependencies for node; great for bootstraping tests on deal making process

## build

```sh
git clone https://github.com/filecoin-project/venus-market.git
cd venus-market
make deps
make
```


## start venus-market

```sh
./venus-market run --node-url <node url> --messager-url <messager-url> --auth-token <auth token>  --signer-url <wallet url> --signer-token  <wallet token> --piecestorage <piece storeage path> --miner <miner address>
```

## start market-client
```shell
./market-client run --node-url <node url> --messager-url <messager-url> --auth-token <auth token>  --signer-url <wallet url> --signer-token  <wallet token> --addr <client default address>
```

## make deal

```shell
 ./market-client generate-car  <file> <car file>
 ./market-client import <file>
 ./market-client deal
```

## retrieval file

```shell
./market-client retrieve --miner <miner addr> <data-cid> <dst path>
```