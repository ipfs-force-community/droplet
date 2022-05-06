#!/bin/sh

echo $@


if [ $nettype=="calibnet" ]
then
    nettype="cali"
fi

echo $nettype
./venus daemon --network=${nettype} --auth-url=http://127.0.0.1:8989 --import-snapshot /snapshot.car


# ./venus-market pool-run \
# --node-url=/ip4/192.168.200.21/tcp/3454/ \
# --auth-url=http://192.168.200.21:8989 \
# --gateway-url=/ip4/192.168.200.21/tcp/45132/ \
# --messager-url=/ip4/192.168.200.21/tcp/39812/ \
# --auth-token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiemwiLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.3u-PInSUmX-8f6Z971M7JBCHYgFVQrvwUjJfFY03ouQ \
# --piecestorage=fs:/path/pieces_solo