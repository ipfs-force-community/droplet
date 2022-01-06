package client

import (
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/imports"
	types2 "github.com/filecoin-project/venus-market/types"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"time"
)

type RestrievalRes struct {
	DealID retrievalmarket.DealID
}

type ClientImportMgr *imports.Manager

type DealInfo struct {
	ProposalCid cid.Cid
	State       storagemarket.StorageDealStatus
	Message     string // more information about deal state, particularly errors
	DealStages  *storagemarket.DealStages
	Provider    address.Address

	DataRef  *storagemarket.DataRef
	PieceCID cid.Cid
	Size     uint64

	PricePerEpoch types.BigInt
	Duration      uint64

	DealID abi.DealID

	CreationTime time.Time
	Verified     bool

	TransferChannelID *datatransfer.ChannelID
	DataTransfer      *types2.DataTransferChannel
}

type StartDealParams struct {
	Data               *storagemarket.DataRef
	Wallet             address.Address
	Miner              address.Address
	EpochPrice         types.BigInt
	MinBlocksDuration  uint64
	ProviderCollateral big.Int
	DealStartEpoch     abi.ChainEpoch
	FastRetrieval      bool
	VerifiedDeal       bool
}

type ImportRes struct {
	Root     cid.Cid
	ImportID imports.ID
}

type RetrievalOrder struct {
	// TODO: make this less unixfs specific
	Root         cid.Cid
	Piece        *cid.Cid
	DataSelector *Selector

	Size  uint64
	Total types.BigInt

	UnsealPrice             types.BigInt
	PaymentInterval         uint64
	PaymentIntervalIncrease uint64
	Client                  address.Address
	Miner                   address.Address
	MinerPeer               *retrievalmarket.RetrievalPeer
}

// Selector specifies ipld selector string
// - if the string starts with '{', it's interpreted as json selector string
//   see https://ipld.io/specs/selectors/ and https://ipld.io/specs/selectors/fixtures/selector-fixtures-1/
// - otherwise the string is interpreted as ipld-selector-text-lite (simple ipld path)
//   see https://github.com/ipld/go-ipld-selector-text-lite
type Selector string

type DagSpec struct {
	// DataSelector matches data to be retrieved
	// - when using textselector, the path specifies subtree
	// - the matched graph must have a single root
	DataSelector *Selector

	// ExportMerkleProof is applicable only when exporting to a CAR file via a path textselector
	// When true, in addition to the selection target, the resulting CAR will contain every block along the
	// path back to, and including the original root
	// When false the resulting CAR contains only the blocks of the target subdag
	ExportMerkleProof bool
}

type ExportRef struct {
	Root cid.Cid

	// DAGs array specifies a list of DAGs to export
	// - If exporting into unixfs files, only one DAG is supported, DataSelector is only used to find the targeted root node
	// - If exporting into a car file
	//   - When exactly one text-path DataSelector is specified exports the subgraph and its full merkle-path from the original root
	//   - Otherwise ( multiple paths and/or JSON selector specs) determines each individual subroot and exports the subtrees as a multi-root car
	// - When not specified defaults to a single DAG:
	//   - Data - the entire DAG: `{"R":{"l":{"none":{}},":>":{"a":{">":{"@":{}}}}}}`
	DAGs []DagSpec

	FromLocalCAR string // if specified, get data from a local CARv2 file.
	DealID       retrievalmarket.DealID
}

type FileRef struct {
	Path  string
	IsCAR bool
}

type CommPRet struct {
	Root cid.Cid
	Size abi.UnpaddedPieceSize
}

type DataSize struct {
	PayloadSize int64
	PieceSize   abi.PaddedPieceSize
}
type DataCIDSize struct {
	PayloadSize int64
	PieceSize   abi.PaddedPieceSize
	PieceCID    cid.Cid
}

type RetrievalInfo struct {
	PayloadCID   cid.Cid
	ID           retrievalmarket.DealID
	PieceCID     *cid.Cid
	PricePerByte abi.TokenAmount
	UnsealPrice  abi.TokenAmount

	Status        retrievalmarket.DealStatus
	Message       string // more information about deal state, particularly errors
	Provider      peer.ID
	BytesReceived uint64
	BytesPaidFor  uint64
	TotalPaid     abi.TokenAmount

	TransferChannelID *datatransfer.ChannelID
	DataTransfer      *types2.DataTransferChannel

	// optional event if part of ClientGetRetrievalUpdates
	Event *retrievalmarket.ClientEvent
}

type QueryOffer struct {
	Err string

	Root  cid.Cid
	Piece *cid.Cid

	Size                    uint64
	MinPrice                types.BigInt
	UnsealPrice             types.BigInt
	PricePerByte            abi.TokenAmount
	PaymentInterval         uint64
	PaymentIntervalIncrease uint64
	Miner                   address.Address
	MinerPeer               retrievalmarket.RetrievalPeer
}

func (o *QueryOffer) Order(client address.Address) RetrievalOrder {
	return RetrievalOrder{
		Root:                    o.Root,
		Piece:                   o.Piece,
		Size:                    o.Size,
		Total:                   o.MinPrice,
		UnsealPrice:             o.UnsealPrice,
		PaymentInterval:         o.PaymentInterval,
		PaymentIntervalIncrease: o.PaymentIntervalIncrease,
		Client:                  client,

		Miner:     o.Miner,
		MinerPeer: &o.MinerPeer,
	}
}

type Import struct {
	Key imports.ID
	Err string

	Root *cid.Cid

	// Source is the provenance of the import, e.g. "import", "unknown", else.
	// Currently useless but may be used in the future.
	Source string

	// FilePath is the path of the original file. It is important that the file
	// is retained at this path, because it will be referenced during
	// the transfer (when we do the UnixFS chunking, we don't duplicate the
	// leaves, but rather point to chunks of the original data through
	// positional references).
	FilePath string

	// CARPath is the path of the CAR file containing the DAG for this import.
	CARPath string
}
