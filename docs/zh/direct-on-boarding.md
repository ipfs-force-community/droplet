## Direct on boarding 使用文档

### 生成订单

#### 使用 droplet-client

1. 生成 car 文件

```
./droplet-client data generate-car droplet droplet.car
```

1. 计算 commp

```
./droplet-client data commP droplet.car

# res

CID:  baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa
Piece size: 254 MiB ( 266338304 B )
```

#### 生成 allocation

```
./droplet-client direct-deal allocate --miner t060973 --wallet t3w6pik7ekdd3nr6slxk4zemau3citbqbdr4yk6qeqxovasltrdki6v7yziyogbklbinujjydxtu6ehetsn42q --piece-info baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa=266338304
```

### 导入订单

```
./droplet storage direct-deal import-deal
```