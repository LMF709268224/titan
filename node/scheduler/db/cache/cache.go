package cache

import (
	"context"
	"time"

	"github.com/linguohua/titan/api"
	"golang.org/x/xerrors"
)

const (
	//CarFileCountField BaseInfo Field
	CarFileCountField = "CarfileCount"
	//DownloadCountField BaseInfo Field
	DownloadCountField = "DownloadCount"
	//NextElectionTimeField BaseInfo Field
	NextElectionTimeField = "NextElectionTime"

	//BlockCountField DeviceInfo Field
	BlockCountField = "BlockCount"
	//TotalDownloadField DeviceInfo Field
	TotalDownloadField = "TotalDownload"
	//TotalUploadField DeviceInfo Field
	TotalUploadField = "TotalUpload"
	//DiskUsageField DeviceInfo Field
	DiskUsageField = "DiskUsage"
)

// DB cache db
type DB interface {
	SetCacheTaskStart(hash, deviceID string, timeout int64) error
	SetCacheTaskEnd(hash, deviceID string) error
	UpdateNodeCacheingExpireTime(hash, deviceID string, timeout int64) error
	// GetCacheingNodes() ([]string, error)
	// GetNodeCaches(deviceID string) ([]string, error)
	GetCacheingCarfiles() (map[string]int, error)
	// GetNodeCacheingCarfile(deviceID string) (string, error)
	NodeIsCaching(deviceID string) (bool, error)

	// waiting data list
	SetWaitingDataTask(info *api.CarfileRecordInfo) error
	GetWaitingDataTask(index int64) (*api.CarfileRecordInfo, error)
	RemoveWaitingDataTasks(infos []*api.CarfileRecordInfo) error

	// validate round id
	IncrValidateRoundID() (int64, error)

	// verifying node list
	SetNodeToVerifyingList(deviceID string) error
	RemoveNodeWithVerifyingList(deviceID string) error
	RemoveVerifyingList() error
	GetNodesWithVerifyingList() ([]string, error)
	CountVerifyingNode(ctx context.Context) (int64, error)

	// validator list
	SetValidatorsToList(deviceIDs []string, expiration time.Duration) error
	GetValidatorsWithList() ([]string, error)
	GetValidatorsAndExpirationTime() ([]string, time.Duration, error)

	// device info
	IncrNodeOnlineTime(deviceID string, onlineTime int64) (float64, error)
	SetDeviceInfo(info *api.DevicesInfo) error
	GetDeviceInfo(deviceID string) (*api.DevicesInfo, error)
	UpdateDeviceInfo(deviceID, field string, value interface{}) error
	IncrByDeviceInfo(deviceID, field string, value int64) error
	UpdateDeviceInfos(field string, values map[string]interface{}) error

	// download info
	SetDownloadBlockRecord(record *DownloadBlockRecord) error
	RemoveDownloadBlockRecord(sn int64) error
	GetDownloadBlockRecord(sn int64) (*DownloadBlockRecord, error)
	IncrBlockDownloadSN() (int64, error)

	// latest data of download
	AddLatestDownloadCarfile(carfileCID string, userIP string) error
	GetLatestDownloadCarfiles(userIP string) ([]string, error)

	NodeDownloadCount(deviceID string, blockDownnloadInfo *api.BlockDownloadInfo) error

	// system base info TODO save in db
	GetBaseInfo() (*api.BaseInfo, error)
	UpdateBaseInfo(field string, value interface{}) error
	IncrByBaseInfo(field string, value int64) error

	// node fid TODO save in db
	// IncrNodeCacheFid(deviceID string, num int) (int, error)
	// GetNodeCacheFid(deviceID string) (int64, error)

	RemoveCarfileRecord(carfileCount int64, nodeBlockCounts map[string]int64) error
	// CacheEndRecord(dataTask *DataTask, fromDeviceID string, blockSize int, blocks int, isSuccess bool) error

	IsNilErr(err error) bool
}

var (
	db         DB
	serverName string
)

// NewCacheDB New Cache DB
func NewCacheDB(url, dbType, sName string) error {
	var err error

	serverName = sName

	switch dbType {
	case TypeRedis():
		db, err = InitRedis(url)
	default:
		err = xerrors.New("unknown CacheDB type")
	}

	return err
}

// GetDB Get CacheDB
func GetDB() DB {
	return db
}

// // NodeInfo base info
// type NodeInfo struct {
// 	OnLineTime int64
// 	LastTime   string
// 	Geo        string
// 	IsOnline   bool
// 	NodeType   api.NodeTypeName
// }

// DataTask data cache task
type DataTask struct {
	CarfileHash string
	DeviceID    string
}

type DownloadBlockRecord struct {
	SN            int64  `redis:"SN"`
	ID            string `redis:"ID"`
	Cid           string `redis:"Cid"`
	UserPublicKey string `redis:"UserPublicKey"`
	NodeStatus    int    `redis:"NodeStatus"`
	UserStatus    int    `redis:"UserStatus"`
	SignTime      int64  `redis:"SignTime"`
	Timeout       int    `redis:"Timeout"`
}
