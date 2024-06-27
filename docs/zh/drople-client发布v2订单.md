## droplet-client 发布 v2 版本订单

使用新的协议 `/fil/storage/mk/1.2.1` 发布非 `DDO` 订单，能支持给 `boost` 发非 `DDO` 订单。

### 发单

```
./droplet-client storage deals init-v2 --provider t060973 --payload-cid bafykbzacedx7l7utesaficnvrcaoqdr5ocon7jrpzxwdhitbeagbwt5jjpums --piece-cid baga6ea4seaqh5pyqze5qhcmjj7ijflqorsyi4n7fcdf6jl5iwz4dqfcnkiss4fy --piece-size 266338304 --duration 518400 --verified-deal --from t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha

# res
sent deal proposal
  deal uuid: b3c379b7-05f2-44cc-9109-b2d308a2f23e
  storage provider: t060973
  client: t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha
  payload cid: bafykbzacedx7l7utesaficnvrcaoqdr5ocon7jrpzxwdhitbeagbwt5jjpums
  commp: baga6ea4seaqh5pyqze5qhcmjj7ijflqorsyi4n7fcdf6jl5iwz4dqfcnkiss4fy
  start epoch: 1760675
  end epoch: 2279075
  provider collateral: 0
  proposal cid: bafyreibyo2kaq34yaczb2tbhqisacihzbf7vgul654om6twppfpagvnktm
```

