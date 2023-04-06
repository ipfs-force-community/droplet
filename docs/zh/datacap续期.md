# datacap 续期

datacap 续期目的是调整 datacap 的最大期限时间，但最大期限时间不能超过 5 年（5256000 epoch）。

## 续期命令

1. 手动指定 claim id，并设置新的最大期限时间和存储提供者（provider）

```
# --from 需使用 datacap 地址，为空则使用 market client 默认地址
# --max-term 新的最大期限时间
./market-client datacap extend --max-term 1909497 --from <address> --claimId 1 <address>

eg.
./market-client datacap extend --max-term 1909497 --from t3wp7bkktkeybm42kvxtyuqsmod262fmwn7zuo3nf3xll34xaokhm4n4x5rgivwg6fcu2mnjecourodjmil3fq --claimId 1 --claimId 2 t01000
```

可以通过命令来查看存储提供者的 claim：
```
./market-client datacap list-claim <address>

eg.
./market-client datacap list-claim t01000
```

2. 自动选择符合要求的 datacap

```
# --from 需使用 datacap 地址，为空则使用 market client 默认地址
# --max-term 新的最大期限时间
# --expiration-cutoff 忽略过期时间小于 cutoff 的 datacap，例：如果 cutoff 的值为 2880（一天），则会对过期时间小于一天的 datacap 进行续期
./market-client datacap extend --max-term 1909697 --from <address> --auto --expiration-cutoff 2880 <address>

eg.
./market-client datacap extend --max-term 1909597 --from t3wp7bkktkeybm42kvxtyuqsmod262fmwn7zuo3nf3xll34xaokhm4n4x5rgivwg6fcu2mnjecourodjmil3fq --auto --expiration-cutoff 2880 t01000
```
