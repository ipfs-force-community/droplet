#!/bin/sh

echo "Init Begin!\n"
echo "set default piece storage path: /PieceStorage\n"
mkdir -p ~/.venusmarket/
if [ ! -f ~/.venusmarket/config.toml ]; then
    echo "set default piece storage path: /PieceStorage"
    cat /docker/config/PieceStorage.toml > ~/.venusmarket/config.toml
fi
echo "Init End!\n"

echo "EXEC: ./venus-market  $@ \n\n"

./venus-market  $@
