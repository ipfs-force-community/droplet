package v230

import (
	"fmt"
	"io"

	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	filestore "github.com/filecoin-project/go-fil-markets/filestore"
	storagemarket "github.com/filecoin-project/go-fil-markets/storagemarket"
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	peer "github.com/libp2p/go-libp2p/core/peer"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var lengthBufMinerDeal = []byte{152, 24}

func (t *MinerDeal) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}

	cw := cbg.NewCborWriter(w)

	if _, err := cw.Write(lengthBufMinerDeal); err != nil {
		return err
	}

	// t.ClientDealProposal (market.ClientDealProposal) (struct)
	if err := t.ClientDealProposal.MarshalCBOR(cw); err != nil {
		return err
	}

	// t.ProposalCid (cid.Cid) (struct)

	if err := cbg.WriteCid(cw, t.ProposalCid); err != nil {
		return xerrors.Errorf("failed to write cid field t.ProposalCid: %w", err)
	}

	// t.AddFundsCid (cid.Cid) (struct)

	if t.AddFundsCid == nil {
		if _, err := cw.Write(cbg.CborNull); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteCid(cw, *t.AddFundsCid); err != nil {
			return xerrors.Errorf("failed to write cid field t.AddFundsCid: %w", err)
		}
	}

	// t.PublishCid (cid.Cid) (struct)

	if t.PublishCid == nil {
		if _, err := cw.Write(cbg.CborNull); err != nil {
			return err
		}
	} else {
		if err := cbg.WriteCid(cw, *t.PublishCid); err != nil {
			return xerrors.Errorf("failed to write cid field t.PublishCid: %w", err)
		}
	}

	// t.Miner (peer.ID) (string)
	if len(t.Miner) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Miner was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.Miner))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Miner)); err != nil {
		return err
	}

	// t.Client (peer.ID) (string)
	if len(t.Client) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Client was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.Client))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Client)); err != nil {
		return err
	}

	// t.State (uint64) (uint64)

	if err := cw.WriteMajorTypeHeader(cbg.MajUnsignedInt, uint64(t.State)); err != nil {
		return err
	}

	// t.PiecePath (filestore.Path) (string)
	if len(t.PiecePath) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.PiecePath was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.PiecePath))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.PiecePath)); err != nil {
		return err
	}

	// t.PayloadSize (uint64) (uint64)

	if err := cw.WriteMajorTypeHeader(cbg.MajUnsignedInt, uint64(t.PayloadSize)); err != nil {
		return err
	}

	// t.MetadataPath (filestore.Path) (string)
	if len(t.MetadataPath) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.MetadataPath was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.MetadataPath))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.MetadataPath)); err != nil {
		return err
	}

	// t.SlashEpoch (abi.ChainEpoch) (int64)
	if t.SlashEpoch >= 0 {
		if err := cw.WriteMajorTypeHeader(cbg.MajUnsignedInt, uint64(t.SlashEpoch)); err != nil {
			return err
		}
	} else {
		if err := cw.WriteMajorTypeHeader(cbg.MajNegativeInt, uint64(-t.SlashEpoch-1)); err != nil {
			return err
		}
	}

	// t.FastRetrieval (bool) (bool)
	if err := cbg.WriteBool(w, t.FastRetrieval); err != nil {
		return err
	}

	// t.Message (string) (string)
	if len(t.Message) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.Message was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.Message))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.Message)); err != nil {
		return err
	}

	// t.FundsReserved (big.Int) (struct)
	if err := t.FundsReserved.MarshalCBOR(cw); err != nil {
		return err
	}

	// t.Ref (storagemarket.DataRef) (struct)
	if err := t.Ref.MarshalCBOR(cw); err != nil {
		return err
	}

	// t.AvailableForRetrieval (bool) (bool)
	if err := cbg.WriteBool(w, t.AvailableForRetrieval); err != nil {
		return err
	}

	// t.DealID (abi.DealID) (uint64)

	if err := cw.WriteMajorTypeHeader(cbg.MajUnsignedInt, uint64(t.DealID)); err != nil {
		return err
	}

	// t.CreationTime (typegen.CborTime) (struct)
	if err := t.CreationTime.MarshalCBOR(cw); err != nil {
		return err
	}

	// t.TransferChannelID (datatransfer.ChannelID) (struct)
	if err := t.TransferChannelID.MarshalCBOR(cw); err != nil {
		return err
	}

	// t.SectorNumber (abi.SectorNumber) (uint64)

	if err := cw.WriteMajorTypeHeader(cbg.MajUnsignedInt, uint64(t.SectorNumber)); err != nil {
		return err
	}

	// t.Offset (abi.PaddedPieceSize) (uint64)

	if err := cw.WriteMajorTypeHeader(cbg.MajUnsignedInt, uint64(t.Offset)); err != nil {
		return err
	}

	// t.PieceStatus (market.PieceStatus) (string)
	if len(t.PieceStatus) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.PieceStatus was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.PieceStatus))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.PieceStatus)); err != nil {
		return err
	}

	// t.InboundCAR (string) (string)
	if len(t.InboundCAR) > cbg.MaxLength {
		return xerrors.Errorf("Value in field t.InboundCAR was too long")
	}

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(t.InboundCAR))); err != nil {
		return err
	}
	if _, err := io.WriteString(w, string(t.InboundCAR)); err != nil {
		return err
	}

	// t.TimeStamp (market.TimeStamp) (struct)
	if err := t.TimeStamp.MarshalCBOR(cw); err != nil {
		return err
	}
	return nil
}

func (t *MinerDeal) UnmarshalCBOR(r io.Reader) (err error) {
	*t = MinerDeal{}

	cr := cbg.NewCborReader(r)

	maj, extra, err := cr.ReadHeader()
	if err != nil {
		return err
	}
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()

	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 24 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.ClientDealProposal (market.ClientDealProposal) (struct)

	{

		if err := t.ClientDealProposal.UnmarshalCBOR(cr); err != nil {
			return xerrors.Errorf("unmarshaling t.ClientDealProposal: %w", err)
		}

	}
	// t.ProposalCid (cid.Cid) (struct)

	{

		c, err := cbg.ReadCid(cr)
		if err != nil {
			return xerrors.Errorf("failed to read cid field t.ProposalCid: %w", err)
		}

		t.ProposalCid = c

	}
	// t.AddFundsCid (cid.Cid) (struct)

	{

		b, err := cr.ReadByte()
		if err != nil {
			return err
		}
		if b != cbg.CborNull[0] {
			if err := cr.UnreadByte(); err != nil {
				return err
			}

			c, err := cbg.ReadCid(cr)
			if err != nil {
				return xerrors.Errorf("failed to read cid field t.AddFundsCid: %w", err)
			}

			t.AddFundsCid = &c
		}

	}
	// t.PublishCid (cid.Cid) (struct)

	{

		b, err := cr.ReadByte()
		if err != nil {
			return err
		}
		if b != cbg.CborNull[0] {
			if err := cr.UnreadByte(); err != nil {
				return err
			}

			c, err := cbg.ReadCid(cr)
			if err != nil {
				return xerrors.Errorf("failed to read cid field t.PublishCid: %w", err)
			}

			t.PublishCid = &c
		}

	}
	// t.Miner (peer.ID) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.Miner = peer.ID(sval)
	}
	// t.Client (peer.ID) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.Client = peer.ID(sval)
	}
	// t.State (uint64) (uint64)

	{

		maj, extra, err = cr.ReadHeader()
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.State = uint64(extra)

	}
	// t.PiecePath (filestore.Path) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.PiecePath = filestore.Path(sval)
	}
	// t.PayloadSize (uint64) (uint64)

	{

		maj, extra, err = cr.ReadHeader()
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.PayloadSize = uint64(extra)

	}
	// t.MetadataPath (filestore.Path) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.MetadataPath = filestore.Path(sval)
	}
	// t.SlashEpoch (abi.ChainEpoch) (int64)
	{
		maj, extra, err := cr.ReadHeader()
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative overflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.SlashEpoch = abi.ChainEpoch(extraI)
	}
	// t.FastRetrieval (bool) (bool)

	maj, extra, err = cr.ReadHeader()
	if err != nil {
		return err
	}
	if maj != cbg.MajOther {
		return fmt.Errorf("booleans must be major type 7")
	}
	switch extra {
	case 20:
		t.FastRetrieval = false
	case 21:
		t.FastRetrieval = true
	default:
		return fmt.Errorf("booleans are either major type 7, value 20 or 21 (got %d)", extra)
	}
	// t.Message (string) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.Message = string(sval)
	}
	// t.FundsReserved (big.Int) (struct)

	{

		if err := t.FundsReserved.UnmarshalCBOR(cr); err != nil {
			return xerrors.Errorf("unmarshaling t.FundsReserved: %w", err)
		}

	}
	// t.Ref (storagemarket.DataRef) (struct)

	{

		b, err := cr.ReadByte()
		if err != nil {
			return err
		}
		if b != cbg.CborNull[0] {
			if err := cr.UnreadByte(); err != nil {
				return err
			}
			t.Ref = new(storagemarket.DataRef)
			if err := t.Ref.UnmarshalCBOR(cr); err != nil {
				return xerrors.Errorf("unmarshaling t.Ref pointer: %w", err)
			}
		}

	}
	// t.AvailableForRetrieval (bool) (bool)

	maj, extra, err = cr.ReadHeader()
	if err != nil {
		return err
	}
	if maj != cbg.MajOther {
		return fmt.Errorf("booleans must be major type 7")
	}
	switch extra {
	case 20:
		t.AvailableForRetrieval = false
	case 21:
		t.AvailableForRetrieval = true
	default:
		return fmt.Errorf("booleans are either major type 7, value 20 or 21 (got %d)", extra)
	}
	// t.DealID (abi.DealID) (uint64)

	{

		maj, extra, err = cr.ReadHeader()
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.DealID = abi.DealID(extra)

	}
	// t.CreationTime (typegen.CborTime) (struct)

	{

		if err := t.CreationTime.UnmarshalCBOR(cr); err != nil {
			return xerrors.Errorf("unmarshaling t.CreationTime: %w", err)
		}

	}
	// t.TransferChannelID (datatransfer.ChannelID) (struct)

	{

		b, err := cr.ReadByte()
		if err != nil {
			return err
		}
		if b != cbg.CborNull[0] {
			if err := cr.UnreadByte(); err != nil {
				return err
			}
			t.TransferChannelID = new(datatransfer.ChannelID)
			if err := t.TransferChannelID.UnmarshalCBOR(cr); err != nil {
				return xerrors.Errorf("unmarshaling t.TransferChannelID pointer: %w", err)
			}
		}

	}
	// t.SectorNumber (abi.SectorNumber) (uint64)

	{

		maj, extra, err = cr.ReadHeader()
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.SectorNumber = abi.SectorNumber(extra)

	}
	// t.Offset (abi.PaddedPieceSize) (uint64)

	{

		maj, extra, err = cr.ReadHeader()
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Offset = abi.PaddedPieceSize(extra)

	}
	// t.PieceStatus (market.PieceStatus) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.PieceStatus = market.PieceStatus(sval)
	}
	// t.InboundCAR (string) (string)

	{
		sval, err := cbg.ReadString(cr)
		if err != nil {
			return err
		}

		t.InboundCAR = string(sval)
	}
	// t.TimeStamp (market.TimeStamp) (struct)

	{

		if err := t.TimeStamp.UnmarshalCBOR(cr); err != nil {
			return xerrors.Errorf("unmarshaling t.TimeStamp: %w", err)
		}

	}
	return nil
}
