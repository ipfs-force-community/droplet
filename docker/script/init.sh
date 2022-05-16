#!/bin/sh

echo "Init Begin!"

echo "args:"
echo $@


echo "set default piece storage path: /PieceStorage"
mkdir -p ~/.venusmarket/
cat /docker/config/PieceStorage.toml > ~/.venusmarket/config.toml
cat  ~/.venusmarket/config.toml
echo "Init End!"

/app/venus-market $@