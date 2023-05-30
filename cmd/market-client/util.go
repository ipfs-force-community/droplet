package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	cli2 "github.com/filecoin-project/venus-market/v2/cli"
	clientapi "github.com/filecoin-project/venus/venus-shared/api/market/client"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
)

func fillDealParams(cctx *cli.Context, p *params, ref *storagemarket.DataRef, miner address.Address) *client.DealParams {
	return &client.DealParams{
		Data:               ref,
		Wallet:             p.from,
		Miner:              miner,
		EpochPrice:         types.BigInt(p.price),
		MinBlocksDuration:  uint64(p.dur),
		DealStartEpoch:     abi.ChainEpoch(cctx.Int64("start-epoch")),
		FastRetrieval:      cctx.Bool("fast-retrieval"),
		VerifiedDeal:       p.isVerified,
		ProviderCollateral: p.provCol,
	}
}

func fillDataRef(cctx *cli.Context,
	api clientapi.IMarketClient,
	cardir string,
	filetype string,
	m *manifest,
	transferType string,
) (*storagemarket.DataRef, error) {
	if m.pieceCID.Defined() {
		return &storagemarket.DataRef{
			TransferType: transferType,
			Root:         m.payloadCID,
			PieceCid:     &m.pieceCID,
			PieceSize:    m.pieceSize,
			RawBlockSize: m.payloadSize,
		}, nil
	}

	fileName := m.payloadCID.String() + ".car"
	if filetype == "piececid" {
		fileName = m.pieceCID.String()
	}

	ref := client.FileRef{
		Path:  filepath.Join(cardir, fileName),
		IsCAR: true,
	}
	ctx := cctx.Context

	res, err := api.ClientImport(ctx, ref)
	if err != nil {
		return nil, err
	}
	encoder, err := cli2.GetCidEncoder(cctx)
	if err != nil {
		return nil, err
	}
	root := encoder.Encode(res.Root)

	if root != m.payloadCID.String() {
		return nil, fmt.Errorf("root not match, expect %s, actual %s", m.payloadCID, root)
	}

	ds, err := api.ClientDealPieceCID(ctx, res.Root)
	if err != nil {
		return nil, err
	}

	return &storagemarket.DataRef{
		TransferType: transferType,
		Root:         res.Root,
		PieceSize:    ds.PieceSize.Unpadded(),
		PieceCid:     &ds.PieceCID,
		RawBlockSize: uint64(ds.PayloadSize),
	}, nil
}

type manifest struct {
	payloadCID  cid.Cid
	payloadSize uint64
	pieceCID    cid.Cid
	pieceSize   abi.UnpaddedPieceSize
}

func loadManifest(path string) ([]*manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}

	manifests := make([]*manifest, 0, len(records))
	for i, record := range records {
		// skip title: payload_cid,filename,piece_cid,payload_size,piece_size,detail or payload_cid,filename,detail
		if i == 0 {
			continue
		}

		if len(record) == 3 {
			payloadCID, err := cid.Parse(record[0])
			if err == nil {
				manifests = append(manifests, &manifest{payloadCID: payloadCID})
			}
		} else if len(record) == 6 {
			payloadCID, err := cid.Parse(record[0])
			if err != nil {
				continue
			}
			pieceCID, err := cid.Parse(record[2])
			if err != nil {
				continue
			}
			payloadSize, err := strconv.ParseUint(record[3], 10, 64)
			if err != nil {
				continue
			}
			pieceSize, err := strconv.Atoi(record[4])
			if err == nil {
				manifests = append(manifests, &manifest{payloadCID: payloadCID, payloadSize: payloadSize,
					pieceCID: pieceCID, pieceSize: abi.UnpaddedPieceSize(pieceSize)})
			}
		}
	}

	return manifests, nil
}

type selector struct {
	pds    map[address.Address]*client.ProviderDistribution
	rds    map[address.Address]*client.ReplicaDistribution
	miners []address.Address

	errs map[address.Address]error
}

func newSelector(dd *client.DealDistribution, miners []address.Address) *selector {
	s := &selector{
		pds:    make(map[address.Address]*client.ProviderDistribution, len(dd.ProvidersDistribution)),
		rds:    make(map[address.Address]*client.ReplicaDistribution, len(dd.ReplicasDistribution)),
		miners: miners,

		errs: make(map[address.Address]error, len(miners)),
	}
	for _, pd := range dd.ProvidersDistribution {
		s.pds[pd.Provider] = pd
	}
	for _, rd := range dd.ReplicasDistribution {
		s.rds[rd.Client] = rd
	}

	return s
}

func (s *selector) selectMiner(clientAddr address.Address, pieceCID cid.Cid, pieceSize uint64) address.Address {
	var foundMiner address.Address
	for _, miner := range s.miners {
		err := s.checkDuplication(miner, pieceCID, pieceSize)
		if err == nil {
			err = s.checkRatio(miner, clientAddr, pieceSize)
		}
		if err != nil {
			foundMiner = miner
			break
		}
		s.errs[miner] = err
	}

	if !foundMiner.Empty() {
		s.update(clientAddr, foundMiner, pieceCID, pieceSize)
	}

	return foundMiner
}

// Storage provider should not be storing duplicate data for more than 20%.
func (s *selector) checkDuplication(miner address.Address, pieceCID cid.Cid, pieceSize uint64) error {
	pd, ok := s.pds[miner]
	if !ok {
		pd = &client.ProviderDistribution{
			Provider:   miner,
			UniqPieces: map[string]uint64{},
		}
	}

	if pd.DuplicationPercentage > 0.2 {
		return fmt.Errorf("duplication percentage %f greater than 0.2", pd.DuplicationPercentage)
	}

	total := pd.Total + pieceSize
	uniq := pd.Uniq
	_, ok = pd.UniqPieces[pieceCID.String()]
	if !ok {
		uniq += pieceSize
	}

	duplicationPercentage := float64(total-uniq) / float64(total)
	if duplicationPercentage > 0.2 {
		return fmt.Errorf("duplication percentage %f greater than 0.2", duplicationPercentage)
	}

	return nil
}

// Storage provider should not exceed 25% of total datacap.
func (s *selector) checkRatio(miner address.Address, clientAddr address.Address, pieceSize uint64) error {
	rd, ok := s.rds[clientAddr]
	if !ok {
		rd = &client.ReplicaDistribution{
			Client:             clientAddr,
			ReplicasPercentage: map[string]float64{},
		}
	}

	perc := rd.ReplicasPercentage[miner.String()]
	if perc > 0.25 {
		return fmt.Errorf("duplication percentage %f greater than 0.25", perc)
	}

	total := rd.Total + pieceSize
	perc = (float64(rd.Total)*perc + float64(pieceSize)) / float64(total)
	if perc > 0.25 {
		return fmt.Errorf("duplication percentage %f greater than 0.25", perc)
	}

	return nil
}

func (s *selector) update(clientAddr address.Address, miner address.Address, pieceCID cid.Cid, pieceSize uint64) {
	pd := s.pds[miner]
	pd.Total += pieceSize
	if _, ok := pd.UniqPieces[pieceCID.String()]; !ok {
		pd.UniqPieces[pieceCID.String()] = pieceSize
		pd.Uniq += pieceSize
	}
	pd.DuplicationPercentage = (float64(pd.Total-pd.Uniq) / float64(pd.Total))

	rd := s.rds[clientAddr]
	total := rd.Total
	rd.Total += pieceSize
	perc := rd.ReplicasPercentage[miner.String()]
	rd.ReplicasPercentage[miner.String()] = (float64(total)*perc + float64(pieceSize)) / float64(rd.Total)
	// todo: Calculate the proportion of each miner ?
}

func (s *selector) printError() {
	fmt.Println("select all miners failed: ")
	for miner, err := range s.errs {
		fmt.Printf("miner: %s, error: %v\n", miner, err)
	}
	fmt.Println()
}
