package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	mbig "math/big"

	eabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// https://github.com/fidlabs/contract-metaallocator/blob/main/src/Client.sol
const contractABI = `[
    {
      "type": "function",
      "name": "allowances",
      "inputs": [
        {
          "name": "client",
          "type": "address",
          "internalType": "address"
        }
      ],
      "outputs": [
        {
          "name": "allowance",
          "type": "uint256",
          "internalType": "uint256"
        }
      ],
      "stateMutability": "view"
    },
    {
      "type": "function",
      "name": "transfer",
      "inputs": [
        {
          "name": "params",
          "type": "tuple",
          "internalType": "struct DataCapTypes.TransferParams",
          "components": [
            {
              "name": "to",
              "type": "tuple",
              "internalType": "struct CommonTypes.FilAddress",
              "components": [
                {
                  "name": "data",
                  "type": "bytes",
                  "internalType": "bytes"
                }
              ]
            },
            {
              "name": "amount",
              "type": "tuple",
              "internalType": "struct CommonTypes.BigInt",
              "components": [
                {
                  "name": "val",
                  "type": "bytes",
                  "internalType": "bytes"
                },
                {
                  "name": "neg",
                  "type": "bool",
                  "internalType": "bool"
                }
              ]
            },
            {
              "name": "operator_data",
              "type": "bytes",
              "internalType": "bytes"
            }
          ]
        }
      ],
      "outputs": [],
      "stateMutability": "nonpayable"
    }
]`

func getAddressAllowanceOnContract(ctx context.Context, api v1.FullNode, wallet address.Address, contract address.Address) (*big.Int, error) {
	// Parse the contract ABI
	parsedABI, err := eabi.JSON(strings.NewReader(contractABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	// Convert from Filecoin to Eth Address
	walletId, err := api.StateLookupID(ctx, wallet, types.EmptyTSK)
	if err != nil {
		return nil, err
	}
	walletEvm, err := types.EthAddressFromFilecoinAddress(walletId)
	if err != nil {
		return nil, err
	}

	// Prepare EVM calldata
	calldata, err := parsedABI.Pack("allowances", walletEvm)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters to check allowances: %w", err)
	}

	// Encode EVM calldata as Message parameters
	allowanceParam := abi.CborBytes(calldata)
	allowanceParams, err := actors.SerializeParams(&allowanceParam)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize params: %w", err)
	}

	// Read allowance from the contract
	allowanceMsg := &types.Message{
		To:     contract,
		From:   wallet,
		Method: builtin.MethodsEVM.InvokeContract,
		Params: allowanceParams,
		Value:  big.Zero(),
	}
	result, err := api.StateCall(ctx, allowanceMsg, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	if result.MsgRct.ExitCode.IsError() {
		return nil, fmt.Errorf("checking allowance failed with ExitCode %d", result.MsgRct.ExitCode)
	}

	// Decode return value (cbor -> evm ABI -> math/big Int -> filecoin big Int)
	var decodedReturn abi.CborBytes
	err = decodedReturn.UnmarshalCBOR(bytes.NewReader(result.MsgRct.Return))
	if err != nil {
		return nil, err
	}
	retValue, err := parsedABI.Unpack("allowances", decodedReturn)
	if err != nil {
		return nil, err
	}
	allowance, err := big.FromString(retValue[0].(*mbig.Int).String())
	return &allowance, err
}

func buildTransferViaEVMParams(amount *big.Int, receiverParams []byte) ([]byte, error) {
	// Parse the contract's ABI
	parsedABI, err := eabi.JSON(strings.NewReader(contractABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	// convert amount from Filecoin big.Int to math/big Int
	var amountMbig mbig.Int
	_, success := amountMbig.SetString(amount.String(), 10)
	if !success {
		return nil, fmt.Errorf("failed to serialize the amount")
	}

	// build calldata
	calldata, err := parsedABI.Pack(
		"transfer",
		TransferParams{
			To: FilAddress{Data: builtin.VerifiedRegistryActorAddr.Bytes()},
			Amount: BigInt{
				Neg: amount.LessThan(big.Zero()),
				Val: amountMbig.Bytes(),
			},
			OperatorData: receiverParams,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize params: %w", err)
	}

	transferParam := abi.CborBytes(calldata)
	transferParams, err := actors.SerializeParams(&transferParam)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize params: %w", err)
	}
	return transferParams, nil
}

// https://github.com/filecoin-project/filecoin-solidity/blob/f655709ab02fcf4b7859f47149f1e0cbfa975041/contracts/v0.8/types/CommonTypes.sol#L86
type FilAddress struct {
	Data []byte
}

// https://github.com/filecoin-project/filecoin-solidity/blob/f655709ab02fcf4b7859f47149f1e0cbfa975041/contracts/v0.8/types/CommonTypes.sol#L80
type BigInt struct {
	Val []byte
	Neg bool
}

// https://github.com/filecoin-project/filecoin-solidity/blob/f655709ab02fcf4b7859f47149f1e0cbfa975041/contracts/v0.8/types/DataCapTypes.sol#L52
type TransferParams struct {
	To           FilAddress
	Amount       BigInt
	OperatorData []byte
}
