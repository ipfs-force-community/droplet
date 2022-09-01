#!/bin/sh

echo "Init Begin!\n"
mkdir -p ~/.venusmarket/
if [ ! -f ~/.venusmarket/config.toml ]; then
    echo "set default piece storage path: /PieceStorage"
    cat /script/config/PieceStorage.toml > ~/.venusmarket/config.toml
fi
echo "Init End!\n"

echo "EXEC: ./venus-market  $@ \n\n"

./venus-market  $@
