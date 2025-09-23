## droplet-client Publish V2 Deals

Using the new protocol `/fil/storage/mk/1.2.1`, you can publish non-`DDO` deals, which makes it possible to send non-`DDO` deals to `boost`.

### Single Deal Proposal

```
./droplet-client storage deals init-v2 --provider t060973 --payload-cid bafykbzacedx7l7utesaficnvrcaoqdr5ocon7jrpzxwdhitbeagbwt5jjpums --piece-cid baga6ea4seaqh5pyqze5qhcmjj7ijflqorsyi4n7fcdf6jl5iwz4dqfcnkiss4fy --piece-size 266338304 --duration 518400 --verified-deal --from t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha

# res
sent deal proposal
  deal uuid: ea06ae14-ba25-49fb-9b1c-25cb7b8360ad
  storage provider: t060973
  client: t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha
  payload cid: bafykbzacedx7l7utesaficnvrcaoqdr5ocon7jrpzxwdhitbeagbwt5jjpums
  piece cid: baga6ea4seaqh5pyqze5qhcmjj7ijflqorsyi4n7fcdf6jl5iwz4dqfcnkiss4fy
  piece size: 268435456
  start epoch: 1761004
  end epoch: 2279404
  provider collateral: 0
```

---

### Batch Deal Proposal

> The `--manifest` flag specifies a file containing piece-related information. Example format:

```
playload_cid,filename,piece_cid,payload_size,piece_size,detail
bafybeibwxlvbeq6jtasa424vngd7q5oa74i3bkdxksyzeopoicb7b7eiki,node-total-54-part-1.car,baga6ea4seaqljycmygqghnweohrswiqozmc5boch6eunbddcvhkcde5tlfmvmki,1048794,2080768,
bafybeibfdgp35tmnqcp3yotqktas3jdenrbq4eesusnlobutib3iva5pqm,node-total-54-part-2.car,baga6ea4seaqod23tw7vaclpmyzx65vxxhja5dmvpytjn2dpg6wtue37ys5rxkgq,1048794,2080768,
```

> The `--output` flag specifies the file where deal information will be saved after proposal.

```
./droplet-client storage deals batch-init-v2 --manifest mm.csv --provider t060973 --duration 518400 --verified-deal --from t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha

# res
created deal 239543ea-99e4-4382-8e7a-a9eb67f3a684 piece cid baga6ea4seaqljycmygqghnweohrswiqozmc5boch6eunbddcvhkcde5tlfmvmki
created deal a7499f28-a8df-407b-a8f0-0f8dde7f0645 piece cid baga6ea4seaqod23tw7vaclpmyzx65vxxhja5dmvpytjn2dpg6wtue37ys5rxkgq
created deal 630591fd-6ecf-4cbc-9229-0f055d9a8015 piece cid baga6ea4seaqkir4npxyxri2fjwcxybxva7t3xsxuzbf43rbil6ulkinjufil6gq
created deal a120b6e8-89ed-4a80-83c6-e41f6c92d059 piece cid baga6ea4seaqfownky2lnb6nk3tjaw7yoeq5butrvl4htulmftv7k7fmb4y7pely
```

An output file will also be generated, for example:
`t060973-2024-06-27-13-31-30.csv`, with the following content:

```
DealUUID,Provider,Client,PieceCID,PieceSize,PayloadCID
239543ea-99e4-4382-8e7a-a9eb67f3a684,t060973,t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha,baga6ea4seaqljycmygqghnweohrswiqozmc5boch6eunbddcvhkcde5tlfmvmki,2097152,bafybeibwxlvbeq6jtasa424vngd7q5oa74i3bkdxksyzeopoicb7b7eiki
a7499f28-a8df-407b-a8f0-0f8dde7f0645,t060973,t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha,baga6ea4seaqod23tw7vaclpmyzx65vxxhja5dmvpytjn2dpg6wtue37ys5rxkgq,2097152,bafybeibfdgp35tmnqcp3yotqktas3jdenrbq4eesusnlobutib3iva5pqm
630591fd-6ecf-4cbc-9229-0f055d9a8015,t060973,t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha,baga6ea4seaqkir4npxyxri2fjwcxybxva7t3xsxuzbf43rbil6ulkinjufil6gq,2097152,bafybeial5ibhe2nue4ohcjyxv6o2ofckumztadt2sy45wsp7gfxf2lea2i
a120b6e8-89ed-4a80-83c6-e41f6c92d059,t060973,t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha,baga6ea4seaqfownky2lnb6nk3tjaw7yoeq5butrvl4htulmftv7k7fmb4y7pely,2097152,bafybeig3nnitacg525c2zgen2ajkbqmvyxvcrgq7ind275zbfswm2z5kam
```
