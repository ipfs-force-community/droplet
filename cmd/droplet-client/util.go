package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/exp/constraints"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
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
		// skip title
		if i == 0 {
			continue
		}

		if len(record) == 3 {
			// payload_cid,filename,detail
			payloadCID, err := cid.Parse(record[0])
			if err == nil {
				manifests = append(manifests, &manifest{payloadCID: payloadCID})
			}
		} else if len(record) == 4 {
			// payload_cid,piece_cid,payload_size,piece_size
			payloadCID, err := cid.Parse(record[0])
			if err != nil {
				continue
			}
			pieceCID, err := cid.Parse(record[1])
			if err != nil {
				continue
			}
			payloadSize, err := strconv.ParseUint(record[2], 10, 64)
			if err != nil {
				continue
			}
			pieceSize, err := strconv.Atoi(record[3])
			if err == nil {
				manifests = append(manifests, &manifest{payloadCID: payloadCID, payloadSize: payloadSize,
					pieceCID: pieceCID, pieceSize: abi.UnpaddedPieceSize(pieceSize)})
			}
		} else if len(record) == 6 {
			// payload_cid,filename,piece_cid,payload_size,piece_size,detail
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
	pds        map[address.Address]*client.ProviderDistribution
	rds        map[address.Address]*client.ReplicaDistribution
	miners     []address.Address
	clientAddr address.Address

	errs map[address.Address]error
}

func newSelector(dd *client.DealDistribution, clientAddr address.Address, miners []address.Address) *selector {
	s := &selector{
		pds:        make(map[address.Address]*client.ProviderDistribution, len(dd.ProvidersDistribution)),
		rds:        make(map[address.Address]*client.ReplicaDistribution, len(dd.ReplicasDistribution)),
		miners:     miners,
		clientAddr: clientAddr,

		errs: make(map[address.Address]error, len(miners)),
	}
	s.init(dd)

	return s
}

func (s *selector) init(dd *client.DealDistribution) {
	for _, pd := range dd.ProvidersDistribution {
		s.pds[pd.Provider] = pd
	}
	for _, miner := range s.miners {
		if _, ok := s.pds[miner]; !ok {
			s.pds[miner] = &client.ProviderDistribution{
				Provider:   miner,
				UniqPieces: make(map[string]uint64),
			}
		}
	}

	for _, rd := range dd.ReplicasDistribution {
		s.rds[rd.Client] = rd
	}
	if _, ok := s.rds[s.clientAddr]; !ok {
		s.rds[s.clientAddr] = &client.ReplicaDistribution{
			Client:             s.clientAddr,
			ReplicasPercentage: map[string]float64{},
		}
	}
}

func (s *selector) selectMiner(pieceCID cid.Cid, pieceSize uint64) address.Address {
	// clean error
	s.errs = make(map[address.Address]error)

	var foundMiner address.Address
	for _, miner := range s.miners {
		err := s.checkDuplication(miner, pieceCID, pieceSize)
		if err == nil {
			err = s.checkRatio(miner, s.clientAddr, pieceSize)
		}
		if err == nil {
			foundMiner = miner
			break
		}
		s.errs[miner] = err
	}

	if !foundMiner.Empty() {
		s.update(s.clientAddr, foundMiner, pieceCID, pieceSize)
	}

	return foundMiner
}

// Storage provider should not be storing duplicate data for more than 20%.
func (s *selector) checkDuplication(miner address.Address, pieceCID cid.Cid, pieceSize uint64) error {
	pd, ok := s.pds[miner]
	if !ok {
		return fmt.Errorf("not found provider distribution")
	}

	total := pd.Total + pieceSize
	uniq := pd.Uniq
	_, ok = pd.UniqPieces[pieceCID.String()]
	if !ok {
		uniq += pieceSize
	}

	duplicationPercentage := float64(total-uniq) / float64(total)
	if duplicationPercentage > 0.2 && duplicationPercentage > pd.DuplicationPercentage {
		return fmt.Errorf("duplication percentage %.2f%s greater than %s", duplicationPercentage*100, "%", "20%")
	}

	return nil
}

// Storage provider should not exceed 25% of total datacap.
func (s *selector) checkRatio(miner address.Address, clientAddr address.Address, pieceSize uint64) error {
	rd, ok := s.rds[clientAddr]
	if !ok {
		return fmt.Errorf("not found replicas distribution: %v", clientAddr)
	}

	oldPercent := rd.ReplicasPercentage[miner.String()]
	total := rd.Total + pieceSize
	// Not checking a small number of deals in the front
	if 10*pieceSize >= total {
		return nil
	}
	percent := (float64(rd.Total)*oldPercent + float64(pieceSize)) / float64(total)
	if percent > 0.25 && percent > oldPercent {
		return fmt.Errorf("replicas percentage %0.2f%s greater than %s", percent*100, "%", "25%")
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
	percent := rd.ReplicasPercentage[miner.String()]
	rd.ReplicasPercentage[miner.String()] = (float64(total)*percent + float64(pieceSize)) / float64(rd.Total)
	// todo: Calculate the proportion of each miner ?
}

func (s *selector) printError() {
	fmt.Println("select all miners failed: ")
	for miner, err := range s.errs {
		fmt.Printf("miner: %s, error: %v\n", miner, err)
	}
	fmt.Println()
}

func getProvidedOrDefaultWallet(ctx context.Context, api clientapi.IMarketClient, addrStr string) (address.Address, error) {
	var a address.Address
	if len(addrStr) != 0 {
		faddr, err := address.NewFromString(addrStr)
		if err != nil {
			return address.Undef, fmt.Errorf("failed to parse 'from' address: %w", err)
		}
		a = faddr
	} else {
		def, err := api.DefaultAddress(ctx)
		if err != nil {
			return address.Undef, err
		}
		a = def
	}

	return a, nil
}

func printJson(obj interface{}) error {
	resJson, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling json: %w", err)
	}

	fmt.Println(string(resJson))
	return nil
}

func Min[T constraints.Ordered](a T, b T) T {
	if a < b {
		return a
	}
	return b
}
