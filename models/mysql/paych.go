package mysql

import (
	"time"

	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/types"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

const (
	channelInfoTableName  = "channel_infos"
	paychMsgInfoTableName = "paych_msg_infos"
)

type channelInfo struct {
	ChannelID     string     `gorm:"column:channel_id;type:varchar(128);primary_key;"`
	Channel       DBAddress  `gorm:"column:channel;type:varchar(256);index"`
	Control       DBAddress  `gorm:"column:control;type:varchar(256);"`
	Target        DBAddress  `gorm:"column:target;type:varchar(256);"`
	Direction     uint64     `gorm:"column:direction;type:bigint unsigned;"`
	NextLane      uint64     `gorm:"column:next_lane;type:bigint unsigned;"`
	Amount        mtypes.Int `gorm:"column:amount;type:varchar(256);"`
	PendingAmount mtypes.Int `gorm:"column:pending_amount;type:varchar(256);"`
	CreateMsg     DBCid      `gorm:"column:create_msg;type:varchar(256);"`
	AddFundsMsg   DBCid      `gorm:"column:add_funds_msg;type:varchar(256);"`
	Settling      bool       `gorm:"column:settling;"`

	VoucherInfo types.VoucherInfos `gorm:"column:voucher_info;type:blob;"`

	IsDeleted bool `gorm:"column:is_deleted;index;default:0;NOT NULL;"`
	TimeStampOrm
}

func (c *channelInfo) TableName() string {
	return channelInfoTableName
}

func fromChannelInfo(src *types.ChannelInfo) *channelInfo {
	info := &channelInfo{
		ChannelID:     src.ChannelID,
		Control:       DBAddress(src.Control),
		Target:        DBAddress(src.Target),
		Direction:     src.Direction,
		NextLane:      src.NextLane,
		Amount:        convertBigInt(src.Amount),
		PendingAmount: convertBigInt(src.PendingAmount),
		Settling:      src.Settling,
		VoucherInfo:   src.Vouchers,
	}
	if src.Channel == nil {
		info.Channel = UndefDBAddress
	} else {
		info.Channel = DBAddress(*src.Channel)
	}
	if src.CreateMsg == nil {
		info.CreateMsg = UndefDBCid
	} else {
		info.CreateMsg = DBCid(*src.CreateMsg)
	}
	if src.AddFundsMsg == nil {
		info.AddFundsMsg = UndefDBCid
	} else {
		info.AddFundsMsg = DBCid(*src.AddFundsMsg)
	}

	return info
}

func toChannelInfo(src *channelInfo) (*types.ChannelInfo, error) {
	info := &types.ChannelInfo{
		ChannelID:     src.ChannelID,
		Channel:       src.Channel.addrPtr(),
		Control:       src.Control.addr(),
		Target:        src.Target.addr(),
		Direction:     src.Direction,
		Vouchers:      src.VoucherInfo,
		NextLane:      src.NextLane,
		Amount:        fbig.Int{Int: src.Amount.Int},
		PendingAmount: fbig.Int{Int: src.PendingAmount.Int},
		CreateMsg:     src.CreateMsg.cidPtr(),
		AddFundsMsg:   src.AddFundsMsg.cidPtr(),
		Settling:      src.Settling,
	}

	return info, nil
}

type channelInfoRepo struct {
	*gorm.DB
}

func NewChannelInfoRepo(db *gorm.DB) *channelInfoRepo {
	return &channelInfoRepo{db}
}

func (c *channelInfoRepo) CreateChannel(from address.Address, to address.Address, createMsgCid cid.Cid, amt fbig.Int) (*types.ChannelInfo, error) {
	ci := &types.ChannelInfo{
		Direction:     types.DirOutbound,
		NextLane:      0,
		Control:       from,
		Target:        to,
		CreateMsg:     &createMsgCid,
		PendingAmount: amt,
	}

	// Save the new channel
	err := c.SaveChannel(ci)
	if err != nil {
		return nil, err
	}

	// Save a reference to the create message
	err = c.DB.Save(fromMsgInfo(&types.MsgInfo{ChannelID: ci.ChannelID, MsgCid: createMsgCid})).Error
	if err != nil {
		return nil, err
	}

	return ci, err
}

func (c *channelInfoRepo) GetChannelByAddress(channel address.Address) (*types.ChannelInfo, error) {
	var info channelInfo
	err := c.DB.Take(&info, "channel = ? and is_deleted = 0", DBAddress(channel).String()).Error
	if err != nil {
		return nil, err
	}

	return toChannelInfo(&info)
}

func (c *channelInfoRepo) GetChannelByChannelID(channelID string) (*types.ChannelInfo, error) {
	var info channelInfo
	err := c.DB.Take(&info, "channel_id = ? and is_deleted = 0", channelID).Error
	if err != nil {
		return nil, err
	}

	return toChannelInfo(&info)
}

func (c *channelInfoRepo) GetChannelByMessageCid(mcid cid.Cid) (*types.ChannelInfo, error) {
	var msgInfo msgInfo
	if err := c.DB.Take(&msgInfo, "msg_cid = ?", DBCid(mcid).String()).Error; err != nil {
		return nil, err
	}

	return c.GetChannelByChannelID(msgInfo.ChannelID)
}

func (c *channelInfoRepo) OutboundActiveByFromTo(from address.Address, to address.Address) (*types.ChannelInfo, error) {
	var ci channelInfo
	err := c.DB.Take(&ci, "direction = ? and settling = ? and control = ? and target = ? and is_deleted = 0",
		types.DirOutbound, false, DBAddress(from).String(), DBAddress(to).String()).Error
	if err != nil {
		return nil, err
	}

	return toChannelInfo(&ci)
}

func (c *channelInfoRepo) WithPendingAddFunds() ([]*types.ChannelInfo, error) {
	var cis []channelInfo
	if err := c.DB.Find(&cis, "direction = ? and is_deleted = 0", types.DirOutbound).Error; err != nil {
		return nil, err
	}
	list := make([]*types.ChannelInfo, 0, len(cis))
	for _, ci := range cis {
		if ci.CreateMsg != UndefDBCid || ci.AddFundsMsg != UndefDBCid {
			ciTmp, err := toChannelInfo(&ci)
			if err != nil {
				return nil, err
			}
			list = append(list, ciTmp)
		}
	}
	return list, nil
}

func (c *channelInfoRepo) ListChannel() ([]address.Address, error) {
	var infos []*channelInfo
	err := c.DB.Find(&infos, "channel != ? and is_deleted = 0", UndefDBAddress.String()).Error
	if err != nil {
		return nil, err
	}
	list := make([]address.Address, 0, len(infos))
	for _, info := range infos {
		if info.Channel == UndefDBAddress {
			continue
		}
		list = append(list, info.Channel.addr())
	}
	return list, nil
}

func (c *channelInfoRepo) SaveChannel(ci *types.ChannelInfo) error {
	if len(ci.ChannelID) == 0 {
		ci.ChannelID = uuid.NewString()
	}
	info := fromChannelInfo(ci)
	info.UpdatedAt = uint64(time.Now().Unix())
	return c.DB.Save(info).Error
}

func (c *channelInfoRepo) RemoveChannel(channelID string) error {
	var info channelInfo
	err := c.DB.Take(&info, "channel_id = ? and is_deleted = 0", channelID).Error
	if err != nil {
		return err
	}
	return c.DB.Model(&channelInfo{}).Where("channel_id = ?", channelID).
		Updates(map[string]interface{}{"is_deleted": 1, "updated_at": time.Now().Unix()}).Error
}

////////// MsgInfo ////////////

type msgInfo struct {
	ChannelID string `gorm:"column:channel_id;type:varchar(256);"`
	MsgCid    DBCid  `gorm:"column:msg_cid;type:varchar(256);primary_key;"`
	Received  bool   `gorm:"column:received;"`
	Err       string `gorm:"column:err;type:varchar(256);"`
	TimeStampOrm
}

func (m *msgInfo) TableName() string {
	return paychMsgInfoTableName
}

func fromMsgInfo(src *types.MsgInfo) *msgInfo {
	return &msgInfo{
		ChannelID: src.ChannelID,
		MsgCid:    DBCid(src.MsgCid),
		Received:  src.Received,
		Err:       src.Err,
	}
}

func toMsgInfo(src *msgInfo) (*types.MsgInfo, error) {
	return &types.MsgInfo{
		ChannelID: src.ChannelID,
		MsgCid:    src.MsgCid.cid(),
		Received:  src.Received,
		Err:       src.Err,
	}, nil
}

type msgInfoRepo struct {
	*gorm.DB
}

func NewMsgInfoRepo(db *gorm.DB) *msgInfoRepo {
	return &msgInfoRepo{db}
}

func (m *msgInfoRepo) GetMessage(mcid cid.Cid) (*types.MsgInfo, error) {
	var info msgInfo
	err := m.DB.Take(&info, "msg_cid = ?", DBCid(mcid).String()).Error
	if err != nil {
		return nil, err
	}
	return toMsgInfo(&info)
}

func (m *msgInfoRepo) SaveMessage(info *types.MsgInfo) error {
	msgInfo := fromMsgInfo(info)
	msgInfo.UpdatedAt = uint64(time.Now().Unix())
	return m.DB.Save(msgInfo).Error
}

func (m *msgInfoRepo) SaveMessageResult(mcid cid.Cid, msgErr error) error {
	cols := make(map[string]interface{})
	cols["updated_at"] = time.Now().Unix()
	cols["received"] = true
	if msgErr != nil {
		cols["err"] = msgErr.Error()
	}
	return m.DB.Model(&msgInfo{}).Where("msg_cid = ?", DBCid(mcid).String()).UpdateColumns(cols).Error
}
