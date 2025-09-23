## Releasing deal Collateral

The **builtin market** requires collateral for deals, but once a deal expires, the collateral is **not automatically released**. You need to manually run a command to release the deal collateral.

```
# Release collateral for a specific deal
./droplet actor-funds settle-deal --miner t060973 100003 100001

# Release collateral for all deals
./droplet actor-funds settle-deal --miner t060973
```
