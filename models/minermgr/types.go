package minermgr

type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Miner      string `json:"miner"` // miner address f01234
	SourceType int    `json:"sourceType"`
	Comment    string `json:"comment"`
	State      int    `json:"state"`
	CreateTime int64  `json:"createTime"`
	UpdateTime int64  `json:"updateTime"`
}
