#!/bin/sh
echo "Compose Init Begin!"

echo $@


token=$(cat /env/token )

echo "token:"
echo ${token}


mkdir -p ~/.venusmarket/
if [ ! -f ~/.venusmarket/config.toml ]; then
    echo "set default piece storage path: /PieceStorage"
    cat /script/config/PieceStorage.toml > ~/.venusmarket/config.toml
fi


echo "Compose Int End!"


/app/venus-market pool-run \
--node-url=/ip4/127.0.0.1/tcp/3453  \
--auth-url=http://127.0.0.1:8989 \
--gateway-url=/ip4/127.0.0.1/tcp/45132/ \
--messager-url=/ip4/127.0.0.1/tcp/39812/ \
--auth-token=${token}
