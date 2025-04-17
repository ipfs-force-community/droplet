package utils

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

type Manifest struct {
	PayloadCID  cid.Cid
	PayloadSize uint64
	PieceCID    cid.Cid
	PieceSize   abi.UnpaddedPieceSize
}

func LoadManifests(path string) ([]Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}

	manifests := make([]Manifest, 0, len(records))
	for i, record := range records {
		// skip title
		if i == 0 && len(record) > 0 && strings.Contains(strings.Join(record, ","), "payload") {
			continue
		}

		if len(record) == 3 {
			// payload_cid,filename,detail
			payloadCID, err := cid.Parse(record[0])
			if err == nil {
				manifests = append(manifests, Manifest{PayloadCID: payloadCID})
			}
		} else if len(record) == 4 {
			// payload_cid,piece_cid,payload_size,piece_size
			payloadCID, err := cid.Parse(record[0])
			if err != nil {
				log.Warnf("failed to parse payload cid %s: %v", record[0], err)
				continue
			}
			pieceCID, err := cid.Parse(record[1])
			if err != nil {
				log.Warnf("failed to parse piece cid %s: %v", record[1], err)
				continue
			}
			payloadSize, err := strconv.ParseUint(record[2], 10, 64)
			if err != nil {
				log.Warnf("failed to parse payload size %s: %v", record[2], err)
				continue
			}
			pieceSize, err := strconv.Atoi(record[3])
			if err == nil {
				manifests = append(manifests, Manifest{PayloadCID: payloadCID, PayloadSize: payloadSize,
					PieceCID: pieceCID, PieceSize: abi.UnpaddedPieceSize(pieceSize)})
			}
		} else if len(record) >= 5 {
			// payload_cid,filename,piece_cid,payload_size,piece_size,detail
			payloadCID, err := cid.Parse(record[0])
			if err != nil {
				log.Warnf("failed to parse payload cid %s: %v", record[0], err)
				continue
			}
			pieceCID, err := cid.Parse(record[2])
			if err != nil {
				log.Warnf("failed to parse piece cid %s: %v", record[2], err)
				continue
			}
			payloadSize, err := strconv.ParseUint(record[3], 10, 64)
			if err != nil {
				log.Warnf("failed to parse payload size %s: %v", record[3], err)
				continue
			}
			pieceSize, err := strconv.Atoi(record[4])
			if err == nil {
				manifests = append(manifests, Manifest{PayloadCID: payloadCID, PayloadSize: payloadSize,
					PieceCID: pieceCID, PieceSize: abi.UnpaddedPieceSize(pieceSize)})
			}
		}
	}

	return manifests, nil
}
