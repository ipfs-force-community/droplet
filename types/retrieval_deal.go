package types

import (
	"fmt"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	xerrors "github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"io"
)

// ProviderDealState is the current state of a deal from the point of view
// of a retrieval provider
type ProviderDealState struct {
	retrievalmarket.DealProposal
	StoreID         uint64
	ChannelID       *datatransfer.ChannelID
	PieceInfo       *piecestore.PieceInfo
	Status          retrievalmarket.DealStatus
	Receiver        peer.ID
	TotalSent       uint64
	FundsReceived   abi.TokenAmount
	Message         string
	CurrentInterval uint64
	LegacyProtocol  bool
}

func (deal *ProviderDealState) IntervalLowerBound() uint64 {
	return deal.Params.IntervalLowerBound(deal.CurrentInterval)
}

func (deal *ProviderDealState) NextInterval() uint64 {
	return deal.Params.NextInterval(deal.CurrentInterval)
}

// Identifier provides a unique id for this provider deal
func (pds ProviderDealState) Identifier() retrievalmarket.ProviderDealIdentifier {
	return retrievalmarket.ProviderDealIdentifier{Receiver: pds.Receiver, DealID: pds.ID}
}

func (t *ProviderDealState) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{171}); err != nil {
		return err
	}

	scratch := make([]byte, 9)

	// t.DealProposal (retrievalmarket.DealProposal) (struct)
	if len("DealProposal") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"DealProposal\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("DealProposal"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("DealProposal")); err != nil {
		return err
	}

	if err := t.DealProposal.MarshalCBOR(w); err != nil {
		return err
	}

	// t.StoreID (uint64) (uint64)
	if len("StoreID") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"StoreID\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("StoreID"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("StoreID")); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.StoreID)); err != nil {
		return err
	}

	// t.ChannelID (datatransfer.ChannelID) (struct)
	if len("ChannelID") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"ChannelID\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("ChannelID"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("ChannelID")); err != nil {
		return err
	}

	if err := t.ChannelID.MarshalCBOR(w); err != nil {
		return err
	}

	// t.PieceInfo (piecestore.PieceInfo) (struct)
	if len("PieceInfo") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"PieceInfo\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("PieceInfo"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("PieceInfo")); err != nil {
		return err
	}

	if err := t.PieceInfo.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Status (retrievalmarket.DealStatus) (uint64)
	if len("Status") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"Status\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("Status"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("Status")); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.Status)); err != nil {
		return err
	}

	// t.Receiver (peer.ID) (string)
	if len("Receiver") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"Receiver\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("Receiver"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("Receiver")); err != nil {
		return err
	}

	if len(t.Receiver) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Receiver was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Receiver))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Receiver)); err != nil {
		return err
	}

	// t.TotalSent (uint64) (uint64)
	if len("TotalSent") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"TotalSent\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("TotalSent"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("TotalSent")); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.TotalSent)); err != nil {
		return err
	}

	// t.FundsReceived (big.Int) (struct)
	if len("FundsReceived") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"FundsReceived\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("FundsReceived"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("FundsReceived")); err != nil {
		return err
	}

	if err := t.FundsReceived.MarshalCBOR(w); err != nil {
		return err
	}

	// t.Message (string) (string)
	if len("Message") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"Message\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("Message"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("Message")); err != nil {
		return err
	}

	if len(t.Message) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Message was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len(t.Message))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Message)); err != nil {
		return err
	}

	// t.CurrentInterval (uint64) (uint64)
	if len("CurrentInterval") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"CurrentInterval\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("CurrentInterval"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("CurrentInterval")); err != nil {
		return err
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajUnsignedInt, uint64(t.CurrentInterval)); err != nil {
		return err
	}

	// t.LegacyProtocol (bool) (bool)
	if len("LegacyProtocol") > cbg.MaxLength {
		return xerrors.Errorf("Value in field \"LegacyProtocol\" was too long")
	}

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, uint64(len("LegacyProtocol"))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string("LegacyProtocol")); err != nil {
		return err
	}

	if err := cbg.WriteBool(w, t.LegacyProtocol); err != nil {
		return err
	}
	return nil
}

func (t *ProviderDealState) UnmarshalCBOR(r io.Reader) error {
	*t = ProviderDealState{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajMap {
		return fmt.Errorf("cbor input should be of type map")
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("ProviderDealState: map struct too large (%d)", extra)
	}

	var name string
	n := extra

	for i := uint64(0); i < n; i++ {

		{
			sval, err := cbg.ReadStringBuf(br, scratch)
			if err != nil {
				return err
			}

			name = string(sval)
		}

		switch name {
		// t.DealProposal (retrievalmarket.DealProposal) (struct)
		case "DealProposal":

			{

				if err := t.DealProposal.UnmarshalCBOR(br); err != nil {
					return xerrors.Errorf("unmarshaling t.DealProposal: %w", err)
				}

			}
			// t.StoreID (uint64) (uint64)
		case "StoreID":

			{

				maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
				if err != nil {
					return err
				}
				if maj != cbg.MajUnsignedInt {
					return fmt.Errorf("wrong type for uint64 field")
				}
				t.StoreID = uint64(extra)

			}
			// t.ChannelID (datatransfer.ChannelID) (struct)
		case "ChannelID":

			{

				b, err := br.ReadByte()
				if err != nil {
					return err
				}
				if b != cbg.CborNull[0] {
					if err := br.UnreadByte(); err != nil {
						return err
					}
					t.ChannelID = new(datatransfer.ChannelID)
					if err := t.ChannelID.UnmarshalCBOR(br); err != nil {
						return xerrors.Errorf("unmarshaling t.ChannelID pointer: %w", err)
					}
				}

			}
			// t.PieceInfo (piecestore.PieceInfo) (struct)
		case "PieceInfo":

			{

				b, err := br.ReadByte()
				if err != nil {
					return err
				}
				if b != cbg.CborNull[0] {
					if err := br.UnreadByte(); err != nil {
						return err
					}
					t.PieceInfo = new(piecestore.PieceInfo)
					if err := t.PieceInfo.UnmarshalCBOR(br); err != nil {
						return xerrors.Errorf("unmarshaling t.PieceInfo pointer: %w", err)
					}
				}

			}
			// t.Status (retrievalmarket.DealStatus) (uint64)
		case "Status":

			{

				maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
				if err != nil {
					return err
				}
				if maj != cbg.MajUnsignedInt {
					return fmt.Errorf("wrong type for uint64 field")
				}
				t.Status = retrievalmarket.DealStatus(extra)

			}
			// t.Receiver (peer.ID) (string)
		case "Receiver":

			{
				sval, err := cbg.ReadStringBuf(br, scratch)
				if err != nil {
					return err
				}

				t.Receiver = peer.ID(sval)
			}
			// t.TotalSent (uint64) (uint64)
		case "TotalSent":

			{

				maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
				if err != nil {
					return err
				}
				if maj != cbg.MajUnsignedInt {
					return fmt.Errorf("wrong type for uint64 field")
				}
				t.TotalSent = uint64(extra)

			}
			// t.FundsReceived (big.Int) (struct)
		case "FundsReceived":

			{

				if err := t.FundsReceived.UnmarshalCBOR(br); err != nil {
					return xerrors.Errorf("unmarshaling t.FundsReceived: %w", err)
				}

			}
			// t.Message (string) (string)
		case "Message":

			{
				sval, err := cbg.ReadStringBuf(br, scratch)
				if err != nil {
					return err
				}

				t.Message = string(sval)
			}
			// t.CurrentInterval (uint64) (uint64)
		case "CurrentInterval":

			{

				maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
				if err != nil {
					return err
				}
				if maj != cbg.MajUnsignedInt {
					return fmt.Errorf("wrong type for uint64 field")
				}
				t.CurrentInterval = uint64(extra)

			}
			// t.LegacyProtocol (bool) (bool)
		case "LegacyProtocol":

			maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
			if err != nil {
				return err
			}
			if maj != cbg.MajOther {
				return fmt.Errorf("booleans must be major type 7")
			}
			switch extra {
			case 20:
				t.LegacyProtocol = false
			case 21:
				t.LegacyProtocol = true
			default:
				return fmt.Errorf("booleans are either major type 7, value 20 or 21 (got %d)", extra)
			}

		default:
			// Field doesn't exist on this type, so ignore it
			cbg.ScanForLinks(r, func(cid.Cid) {})
		}
	}
	return nil
}
