# Deal Filter

## Background

Sometimes, `SP` may want more fine-grained control over whether to accept deals and which deals to accept. For example, some `SP` may want to only accept deals from specific `peer`, or only accept deals from specific `peer` where the deal price must be within a certain range.

## Details

To meet these requirements, an deal filter can be configured for a specific `miner` in the `Droplet` configuration file. This filter is represented in the configuration file as a string indicating a `shell` command. Whenever `Droplet` decides whether to accept an deal directed to a certain `miner`, it will call this command and pass the `deal information` (a JSON string) as a parameter (standard input) to the command. If the command exits with code `0`, it means the deal is accepted; otherwise, the deal is rejected.

- exit with 0: Accept the deal
- exit with non-0: Reject the deal

### Deal Information

- Storage Deal

```json
{
"IsOffline": false,
"FormatVersion":      "1.0.0",
"FastRetrieval":        false,
"TransferType" :        "manual",
"ClientDealProposal":{
    "Proposal": {
    "PieceCID": {
    "/": "baga6ea4seaqihx2pxanewwxvqwgeyrcmal7aomucelef52vhqy7qaarciamaqoq"
    },
    "PieceSize": 2048,
    "VerifiedDeal": false,
    "Client": "f3r3hr3xl27unpefvipve2f4hlfvdnq3forgr253z6dqahufvanatdandxm74zikheccvx74ys7by5vzafq2va",
    "Provider": "f01000",
    "Label": "bafk2bzacebiupsywspqnsvc5v7ing74i3u4y3r7wtgjioor7pqn3cxopq7lo4",
    "StartEpoch": 18171,
    "EndEpoch": 536571,
    "StoragePricePerEpoch": "1",
    "ProviderCollateral": "0",
    "ClientCollateral": "0"
    },
    "ClientSignature": {
    "Type": 2,
    "Data": "oEnUUL1WejrLawl3sP9o/TZYRZgPYA86xmF3RMQt5bPQJbrK/5x3UXYxeUKoIDMjE96fA1GSqfrE14tFl/nMyatPLUvzzZ0ulsPTQVwfb54Mgx0yBSMYTf/O8Bg09MNq"
    },
},
"DealType": "storage",
"Agent": "droplet"
}
```

- Retrieval Deal

```json
{
  "PayloadCID": null,
  "ID": 0,
  "Selector": null,
  "PieceCID": null,
  "PricePerByte": "\u003cnil\u003e",
  "PaymentInterval": 0,
  "PaymentIntervalIncrease": 0,
  "UnsealPrice": "\u003cnil\u003e",
  "StoreID": 0,
  "SelStorageProposalCid": null,
  "ChannelID": null,
  "Status": 0,
  "Receiver": "",
  "TotalSent": 0,
  "FundsReceived": "\u003cnil\u003e",
  "Message": "",
  "CurrentInterval": 0,
  "LegacyProtocol": false,
  "CreatedAt": 0,
  "UpdatedAt": 0,
  "DealType": "retrieval"
}
```

## Examples

```toml
# Storage Deal
Filter = ""

# Retrieval Deal
RetrievalFilter = ""
```

- Example: The simplest deal filter

```toml
# Reject all deals
Filter = "exit 1"

# Accept all deals
Filter = "exit 0"
```

- Example: Only accept deals from `f01000`

```toml
Filter = "jq -r '.ClientDealProposal.Proposal.Provider' | grep -q '^f01000$'"
```

- Example: Only accept deals sent from addresses `f1aaaaaaaaaaaaaaaaaaaaaaaaa`, `f1bbbbbbbbbbbbbbbbbbbbbbbbb`, and `f1ccccccccccccccccccccccccc`

```toml
Filter = "jq -e '.Proposal.Client == \"f1aaaaaaaaaaaaaaaaaaaaaaaaa\" or .Proposal.Client == \"f1bbbbbbbbbbbbbbbbbbbbbbbbb\" or .Proposal.Client == \"f1ccccccccccccccccccccccccc\"'"
```

- Example: Only accept Verified deals

```
Filter = "jq -e '.ClientDealProposal.Proposal.VerifiedDeal == true'"
```

- Example: Using a `python` script

```toml
# config.toml
Filter = "python3 /path/to/filter.py"
```

```python
# filter.py
import json
import sys

try:
    json_str = sys.stdin.read()
    data = json.loads(json_str)

    if data["ClientDealProposal"]['Proposal']['PieceSize'] < 2048:
        print("")
        sys.exit(0)
    else:
        print("PieceSize is greater than or equal to 2048. Exiting with code 1.")
        sys.exit(1)
except Exception as e:
    print("An error occurred: ", e)
    sys.exit(1)
```
