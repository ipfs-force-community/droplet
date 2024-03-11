## Direct on boarding 使用文档

### 生成单个订单

1. 生成 car 文件

```
./droplet-client data generate-car droplet droplet.car
```

2. 计算 commp

```
./droplet-client data commP droplet.car

# res
CID:  baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa
Piece size: 254 MiB ( 266338304 B )
```

3. 生成 allocation

```
./droplet-client direct-deal allocate --miner t060973 --wallet t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha --piece-info baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa=266338304

# res

submitted data cap allocation message: bafy2bzacec6nhbzd4zu3zucyueivaffdnabnnhg7ztjcug5d4gsuuikexzigw
waiting for message to be included in a block

AllocationID  Client    Miner    PieceCid                                                          PieceSize  TermMin  TermMax  Expiration
31649         t018678   t060973  baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa  266338304  518400   777600   1406893
```

#### 导入订单

```
./droplet storage direct-deal import-deal --allocation-id 31649 --client t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa droplet.car
```

### 生成多个订单

#### 生成 piece 文件

1. 使用 [go-graphsplit](https://github.com/filedrive-team/go-graphsplit) 生成 piece。

```
./graphsplit chunk --slice-size 1048576 --car-dir data2 --calc-commp --rename --graph-name node graphsplit
```

执行完成后会在 `data2` 生成 piece 文件及 `manifest.csv`，`manifest.csv` 包含 piece 的基本信息，也可以用于批量发布订单。

2. 批量生成订单

```
./droplet-client direct-deal allocate --miner t060973 --wallet t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha --manifest ./data2/manifest.csv

# res
submitted data cap allocation message: bafy2bzacebvln7ksauj2tsnoe7vmnhwooc7ovkwov4ajhh57fv6r6yws35nmo
waiting for message to be included in a block

AllocationID  Client    Miner    PieceCid                                                          PieceSize  TermMin  TermMax  Expiration
31650         t018678   t060973  baga6ea4seaqod23tw7vaclpmyzx65vxxhja5dmvpytjn2dpg6wtue37ys5rxkgq  2097152    518400   777600   1386797
31651         t018678   t060973  baga6ea4seaqkir4npxyxri2fjwcxybxva7t3xsxuzbf43rbil6ulkinjufil6gq  2097152    518400   777600   1386797
31652         t018678   t060973  baga6ea4seaqfownky2lnb6nk3tjaw7yoeq5butrvl4htulmftv7k7fmb4y7pely  2097152    518400   777600   1386797
31653         t018678   t060973  baga6ea4seaqnjeqzphriivug6ua4ehibqm62hi2uky6rhh4xzsg47owhkpotgfq  2097152    518400   777600   1386797
31654         t018678   t060973  baga6ea4seaqgozgsl7ddfjqig6za3l7o5sf6oiw5hd4ggug7tiqfhi5gajwq4ja  2097152    518400   777600   1386797
```

3. 批量导入订单

```
./droplet storage direct-deal import-deals --allocation-file allocation.csv --car-dir ./data2/

# res
import deal success
```

导入订单命令提供了几个可选的 flag：

* --no-copy-car-file 不拷贝 car 文件到 piece storage，也不会计算 commp 和生成索引
* --skip-commp 不计算 commp，不能保证导入的订单数据是否正确
* --skip-index 不生成索引
