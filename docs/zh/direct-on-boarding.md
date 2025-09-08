## Direct on boarding 使用文档

### 生成单个订单

#### 生成 piece 文件

```
./droplet-client data generate-car droplet droplet.car
```

#### 计算 commP

```
./droplet-client data commP droplet.car

# res
CID:  baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa
Piece size: 254 MiB ( 266338304 B )
```

#### 生成订单

```
./droplet-client direct-deal allocate --miner t060973 --wallet t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha --piece-info baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa=266338304

# res

submitted data cap allocation message: bafy2bzacec6nhbzd4zu3zucyueivaffdnabnnhg7ztjcug5d4gsuuikexzigw
waiting for message to be included in a block

AllocationID  Client    Miner    PieceCid                                                          PieceSize  TermMin  TermMax  Expiration
31649         t018678   t060973  baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa  266338304  518400   777600   1406893
```

#### 导入单个订单

flag 解释：

* --skip-commp 跳过计算验证 piece cid，可以减少导入时间

```
./droplet storage direct-deal import-deal --allocation-id 31649 --client t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha baga6ea4seaqconolebafjmjlqc35z4foyzfipxfuiav25okti22kjof7rbgoipa droplet.car
```

### 生成多个订单

#### 使用 [go-graphsplit](https://github.com/filedrive-team/go-graphsplit) 生成 piece。

flag 解释：

* --slice-size piece 文件大小
* --car-dir 用于存储生成的 piece 文件的目录
* --calc-commp 计算 piece cid
* --rename 用 piece cid 作为文件名，方便后续使用
* --graph-name 可随意命名，暂无实际作用

```
./graphsplit chunk --slice-size 1048576 --car-dir data2 --calc-commp --rename --graph-name node graphsplit
```

执行完成后会在 `data2` 生成 piece 文件及 `manifest.csv`，`manifest.csv` 包含 piece 的基本信息，也可以用于批量发布订单。

#### 批量生成订单

flag 解释：

--wallet 发单地址，需要有 DC
--manifest 由 graphsplit 生成
--output-allocation-to-file 用于保存新生成的 allocation 信息，可以用批量导入订单
--droplet-url droplet url，如果设置了，则自动导入新生成的订单到 droplet，无需执行后续的 `批量导入订单`
--droplet-token droplet token

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

#### 通过合约发布订单

新增加 --evm-client-contract 来指定 client 合约地址

```
./droplet-client direct-deal allocate --miner t060973 --wallet t3wivhkdivcxj5zp2l4wjkzon232s52smnd5m3na66ujl5nel75jggguhgaa3zbhjo3as4epf5ytxl6ly3qoha --manifest ./data2/manifest.csv --evm-client-contract f04xxxxx
```

#### 批量导入订单

flag 解释：

* --skip-commp 跳过计算验证 piece cid，可以减少导入时间

```
./droplet storage direct-deal import-deals --allocation-file allocation.csv

# res
import deal success
```

### 查询订单信息

#### 查询单个订单

```
./droplet storage direct-deal get --allocation-id 32224
or
./droplet storage direct-deal get --id 07cd5814-02bf-494d-b81c-87df73b3422b

# res
Creation     2024-04-01T14:30:39+08:00
PieceCID     baga6ea4seaqgzkse45r2tinm4cy7pducjt45c2r77qnu4r6uytxlsiazev6xwpy
PieceSize    2097152
Client       t018678
Provider     t060973
AllocationID 32224
State        DealAllocated
Message
SectorID     0
Offset       0
Length       0
PayloadSize  1048794
StartEpoch   1510595
EndEpoch     2028995
```

#### 列出订单

> 该命令默认只会列出 DealAllocated 状态的订单，可以通过 --state flag 指定特定状态的订单

```
./droplet storage direct-deal list

# res
Creation                   ID                                    AllocationId  PieceCid                                                          State          Client   Provider  Size     Message
2024-04-01T14:30:40+08:00  07cd5814-02bf-494d-b81c-87df73b3422b  32227         baga6ea4seaqhbpwuqszynr4wmtn2osjwkru3nrp6z6bjte6c4rzlntfm4l5s2ia  DealAllocated  t018678  t060973   1048576
2024-04-01T14:30:40+08:00  aeaf18c5-2d92-4376-9370-594b8536190f  32226         baga6ea4seaqeddngzgmtkxa3wqetu27j5ydqie7hwa5rjrl5de6osamz3ldpegi  DealAllocated  t018678  t060973   2097152
2024-04-01T14:30:39+08:00  d94f289d-50be-460b-8555-8b9a398e35d6  32223         baga6ea4seaqp6jm4x3pf7llach7tdhbwrwlcetv52dnjicqpcl6lkwib5n76gii  DealAllocated  t018678  t060973   2097152
2024-04-01T14:30:39+08:00  ea01b452-109f-4373-ab80-6af76d75b6d6  32224         baga6ea4seaqgzkse45r2tinm4cy7pducjt45c2r77qnu4r6uytxlsiazev6xwpy  DealAllocated  t018678  t060973   2097152
2024-04-01T14:30:39+08:00  f82a5335-6036-4bb5-9d6b-d31030cb3272  32225         baga6ea4seaqdddnss5oalozaqsogmhfohxh2hyclw5ewxdgdme53tn3pidzgeeq  DealAllocated  t018678  t060973   2097152
```

#### 更新订单状态

```
./droplet storage direct-deal update-state --state 1 07cd5814-02bf-494d-b81c-87df73b3422b
```

### 从消息导入订单

发送订单时程序退出，但订单没有导入到 `droplet`，这种情况可以从消息里面获取订单信息并导入到 `droplet`。

```
./droplet storage direct-deal import-deals-from-msg --msg <msg cid> --manifest <manifest> --skip-commp
```

### 更新订单 payload cid

```
./droplet storage direct-deal update-payload-cid --manifest <manifest>
```
