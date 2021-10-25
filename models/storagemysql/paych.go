package storagemysql

import (
	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus-market/types"
	mtypes "github.com/filecoin-project/venus-messager/types"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type channelInfo struct {
	ChannelID     string     `gorm:"column:channel_id;type:varchar(256);primary_key;"`
	Channel       string     `gorm:"column:channel;type:varchar(256);"`
	Control       string     `gorm:"column:control;type:varchar(256);"`
	Target        string     `gorm:"column:target;type:varchar(256);"`
	Direction     uint64     `gorm:"column:direction;type:bigint unsigned;"`
	NextLane      uint64     `gorm:"column:next_lane;type:bigint unsigned;"`
	Amount        mtypes.Int `gorm:"column:amount;type:varchar(256);"`
	PendingAmount mtypes.Int `gorm:"column:pending_amount;type:varchar(256);"`
	CreateMsg     string     `gorm:"column:create_msg;type:varchar(256);"`
	AddFundsMsg   string     `gorm:"column:add_funds_msg;type:varchar(256);"`
	Settling      bool       `gorm:"column:settling;"`

	VoucherInfo []*types.VoucherInfo `gorm:"column:voucher_info;type:blob;"`

	IsDeleted int `gorm:"column:is_deleted;index;default:-1;NOT NULL"`
}

func (c *channelInfo) TableName() string {
	return "channel_infos"
}

func fromChannelInfo(src *types.ChannelInfo) *channelInfo {
	info := &channelInfo{
		ChannelID:   src.ChannelID,
		Channel:     src.Channel.String(),
		Control:     src.Control.String(),
		Target:      src.Target.String(),
		Direction:   src.Direction,
		NextLane:    src.NextLane,
		CreateMsg:   src.CreateMsg.String(),
		AddFundsMsg: src.AddFundsMsg.String(),
		Settling:    src.Settling,
		VoucherInfo: src.Vouchers,
	}
	if !src.Amount.Nil() {
		info.Amount = mtypes.NewFromGo(src.Amount.Int)
	} else {
		info.Amount = mtypes.Zero()
	}
	if !src.PendingAmount.Nil() {
		info.PendingAmount = mtypes.NewFromGo(src.PendingAmount.Int)
	} else {
		info.PendingAmount = mtypes.Zero()
	}
	return info
}

func toChannelInfo(src *channelInfo) (*types.ChannelInfo, error) {
	info := &types.ChannelInfo{
		ChannelID:     src.ChannelID,
		Channel:       nil,
		Control:       address.Address{},
		Target:        address.Address{},
		Direction:     src.Direction,
		Vouchers:      src.VoucherInfo,
		NextLane:      src.NextLane,
		Amount:        fbig.Int{Int: src.Amount.Int},
		PendingAmount: fbig.Int{Int: src.PendingAmount.Int},
		CreateMsg:     nil,
		AddFundsMsg:   nil,
		Settling:      src.Settling,
	}
	var err error
	channel, err := address.NewFromString(src.Channel)
	if err != nil {
		return nil, err
	}
	info.Channel = &channel
	info.Control, err = address.NewFromString(src.Control)
	if err != nil {
		return nil, err
	}
	info.Target, err = address.NewFromString(src.Target)
	if err != nil {
		return nil, err
	}
	createMsg, err := parseCid(src.CreateMsg)
	if err != nil {
		return nil, err
	}
	info.CreateMsg = &createMsg
	addFundsMsg, err := parseCid(src.AddFundsMsg)
	if err != nil {
		return nil, err
	}
	info.AddFundsMsg = &addFundsMsg

	return info, nil
}

type channelInfoRepo struct {
	*gorm.DB
}

func newChannelInfoRepo(db *gorm.DB) *channelInfoRepo {
	return &channelInfoRepo{db}
}

func (c *channelInfoRepo) GetChannelByAddress(channel address.Address) (*types.ChannelInfo, error) {
	var info channelInfo
	err := c.DB.Take(&info, "channel = ? and is_deleted = -1", channel.String()).Error
	if err != nil {
		return nil, err
	}

	return toChannelInfo(&info)
}

func (c *channelInfoRepo) GetChannelByChannelID(channelID string) (*types.ChannelInfo, error) {
	var info channelInfo
	err := c.DB.Take(&info, "channel_id = ? and is_deleted = -1", channelID).Error
	if err != nil {
		return nil, err
	}

	return toChannelInfo(&info)
}

func (c *channelInfoRepo) GetChannelByMessageCid(mcid cid.Cid) (*types.ChannelInfo, error) {
	var info channelInfo
	err := c.DB.Take(&info, "create_msg = ? and is_deleted = -1", mcid.String()).Error
	if err != nil {
		return nil, err
	}

	return toChannelInfo(&info)
}

func (c *channelInfoRepo) ListChannel() ([]*types.ChannelInfo, error) {
	var infos []*channelInfo
	err := c.DB.Find(&infos, "is_deleted = -1").Error
	if err != nil {
		return nil, err
	}
	list := make([]*types.ChannelInfo, 0, len(infos))
	for _, info := range infos {
		tmpInfo, err := toChannelInfo(info)
		if err != nil {
			return nil, err
		}
		list = append(list, tmpInfo)
	}
	return list, nil
}

func (c *channelInfoRepo) SaveChannel(ci *types.ChannelInfo) error {
	return c.DB.Create(fromChannelInfo(ci)).Error
}

func (c *channelInfoRepo) RemoveChannel(channelID string) error {
	var info channelInfo
	err := c.DB.Take(info, "channel_id = ? and is_deleted = -1", channelID).Error
	if err != nil {
		return err
	}
	return c.DB.Model(&channelInfo{}).Where("channel_id = ?", channelID).Update("is_deleted", 1).Error
}

////////// MsgInfo ////////////

type msgInfo struct {
	ChannelID string `gorm:"column:channel_id;type:varchar(256);"`
	MsgCid    string `gorm:"column:msg_cid;type:varchar(256);primary_key;"`
	Received  bool   `gorm:"column:received;"`
	Err       string `gorm:"column:err;type:varchar(256);"`
}

func (m *msgInfo) TableName() string {
	return "paych_msg_infos"
}

func fromMsgInfo(src *types.MsgInfo) *msgInfo {
	return &msgInfo{
		ChannelID: src.ChannelID,
		MsgCid:    src.MsgCid.String(),
		Received:  src.Received,
		Err:       src.Err,
	}
}

func toMsgInfo(src *msgInfo) (*types.MsgInfo, error) {
	info := &types.MsgInfo{
		ChannelID: src.ChannelID,
		MsgCid:    cid.Cid{},
		Received:  src.Received,
		Err:       src.Err,
	}
	var err error
	info.MsgCid, err = parseCid(src.MsgCid)
	if err != nil {
		return nil, err
	}

	return info, nil
}

type msgInfoRepo struct {
	*gorm.DB
}

func newMsgInfoRepo(db *gorm.DB) *msgInfoRepo {
	return &msgInfoRepo{db}
}

func (m *msgInfoRepo) GetMessage(mcid cid.Cid) (*types.MsgInfo, error) {
	var info msgInfo
	err := m.DB.Take(&info, "msg_cid = ?", mcid.String()).Error
	if err != nil {
		return nil, err
	}
	return toMsgInfo(&info)
}

func (m *msgInfoRepo) SaveMessage(info *types.MsgInfo) error {
	return m.DB.Create(fromMsgInfo(info)).Error
}

func (m *msgInfoRepo) SaveMessageResult(mcid cid.Cid, msgErr error) error {
	return m.DB.Model(&msgInfo{}).Where("msg_cid = ?", mcid.String()).Update("err", msgErr).Error
}
