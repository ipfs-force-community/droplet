- [ ] use venus api `StateGetNetworkParams` to detect network name at: api/clients/modules.go:74
- [ ] protect peer-id, when connect to a venus node in mesh: indexprovider/mesh.go:28 


```shell
 curl http://localhost:41235/rpc/v0 -X POST -H "Content-Type: application/json"  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiTWFya2V0TG9jYWxUb2tlbiIsInBlcm0iOiJhZG1pbiIsImV4dCI6IiJ9.ediIdhR5zg4cCeYLj8AAisEefEflm5mTdS4BeZ_wurU" -H "X-Venus-Api-Namespace: market.IMarket"  -d '{"jsonrpc":"2.0","id":1,"method":"VENUS_MARKET.NetConnect","params":["/ip4/192.168.1.125/tcp/43498/p2p/QmaLGqBTDiA3nwUXbLCvRyAy4UobwsGT8AEwHz6Yqv9248"],"meta":{"SpanContext":"AAD6kRGJ8L4rqWx0jBGnvwhsAe0fHmXKcP+xAgA="}}'
```