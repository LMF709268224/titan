package api

import (
	"context"
	"time"

	"github.com/filecoin-project/go-jsonrpc/auth"
)

// Scheduler Scheduler node
type Scheduler interface {
	Common
	Web

	// call by command
	GetOnlineDeviceIDs(ctx context.Context, nodeType NodeType) ([]string, error)                //perm:read
	ElectionValidators(ctx context.Context) error                                               //perm:admin
	CacheCarfile(ctx context.Context, info *CacheCarfileInfo) error                             //perm:admin
	RemoveCarfile(ctx context.Context, carfileID string) error                                  //perm:admin
	RemoveCache(ctx context.Context, carfileID, deviceID string) error                          //perm:admin
	GetCarfileRecordInfo(ctx context.Context, cid string) (CarfileRecordInfo, error)            //perm:read
	ListCarfileRecords(ctx context.Context, page int) (*DataListInfo, error)                    //perm:read
	GetRunningCarfileRecords(ctx context.Context) ([]*CarfileRecordInfo, error)                 //perm:read
	RegisterNode(ctx context.Context, nodeType NodeType, count int) ([]NodeRegisterInfo, error) //perm:admin
	ValidateSwitch(ctx context.Context, open bool) error                                        //perm:admin
	ValidateRunningState(ctx context.Context) (bool, error)                                     //perm:admin
	ValidateStart(ctx context.Context) error                                                    //perm:admin
	ResetCacheExpiredTime(ctx context.Context, carfileCid string, expiredTime time.Time) error  //perm:admin
	NodeQuit(ctx context.Context, device string) error                                          //perm:admin
	StopCacheTask(ctx context.Context, carfileCid string) error                                 //perm:admin
	ResetBackupCacheCount(ctx context.Context, backupCacheCount int) error                      //perm:admin
	GetUndoneCarfileRecords(ctx context.Context, page int) (*DataListInfo, error)               //perm:read
	ExecuteUndoneCarfilesTask(ctx context.Context) error                                        //perm:admin
	ShowNodeLogFile(ctx context.Context, deviceID string) (*LogFile, error)                     //perm:admin
	DownloadNodeLogFile(ctx context.Context, deviceID string) ([]byte, error)                   //perm:admin
	DeleteNodeLogFile(ctx context.Context, deviceID string) error                               //perm:admin
	// call by locator
	LocatorConnect(ctx context.Context, locatorID, locatorToken string) error //perm:write

	// call by node
	// node send result when user download block complete
	NodeResultForUserDownloadBlock(ctx context.Context, result NodeBlockDownloadResult) error                //perm:write
	EdgeNodeConnect(ctx context.Context) error                                                               //perm:write
	ValidateBlockResult(ctx context.Context, validateResults ValidateResults) error                          //perm:write
	CandidateNodeConnect(ctx context.Context) error                                                          //perm:write
	CacheResult(ctx context.Context, resultInfo CacheResultInfo) error                                       //perm:write
	RemoveCarfileResult(ctx context.Context, resultInfo RemoveCarfileResultInfo) error                       //perm:write
	GetExternalAddr(ctx context.Context) (string, error)                                                     //perm:read
	GetPublicKey(ctx context.Context) (string, error)                                                        //perm:write
	AuthNodeVerify(ctx context.Context, token string) ([]auth.Permission, error)                             //perm:read
	AuthNodeNew(ctx context.Context, perms []auth.Permission, deviceID, deviceSecret string) ([]byte, error) //perm:read

	GetNodeAppUpdateInfos(ctx context.Context) (map[int]*NodeAppUpdateInfo, error) //perm:read
	SetNodeAppUpdateInfo(ctx context.Context, info *NodeAppUpdateInfo) error       //perm:admin                                                           //perm:write
	DeleteNodeAppUpdateInfos(ctx context.Context, nodeType int) error              //perm:admin

	// nat travel
	GetAllEdgeAddrs(ctx context.Context) (map[string]string, error) //perm:write
	// nat travel, can get edge external addr with different scheduler
	GetEdgeExternalAddr(ctx context.Context, deviceID, schedulerURL string) (string, error) //perm:write
	// nat travel
	CheckEdgeIfBehindFullConeNAT(ctx context.Context, edgeURL string) (bool, error) //perm:read

	// call by user
	GetDownloadInfosWithCarfile(ctx context.Context, cid, publicKey string) ([]*DownloadInfoResult, error) //perm:read
	GetDevicesInfo(ctx context.Context, deviceID string) (DevicesInfo, error)                              //perm:read
	GetDownloadInfo(ctx context.Context, deviceID string) ([]*BlockDownloadInfo, error)                    //perm:read

	// user send result when user download block complete or failed
	UserDownloadBlockResults(ctx context.Context, results []UserBlockDownloadResult) error //perm:read
}

// DataListInfo Data List Info
type DataListInfo struct {
	Page           int
	TotalPage      int
	Cids           int
	CarfileRecords []*CarfileRecordInfo
}

// CacheEventInfo Event Info
type CacheEventInfo struct {
	ID   int
	CID  string    `db:"cid"`
	Msg  string    `db:"msg"`
	Time time.Time `db:"time"`
}

// EventListInfo Event List Info
type EventListInfo struct {
	Page      int
	TotalPage int
	Count     int
	EventList []*CacheEventInfo
}

// NodeRegisterInfo Node Register Info
type NodeRegisterInfo struct {
	ID         int
	DeviceID   string `db:"device_id"`
	Secret     string `db:"secret"`
	CreateTime string `db:"create_time"`
	NodeType   int    `db:"node_type"`
}

// CacheResultInfo cache data result info
type CacheResultInfo struct {
	Status            CacheStatus
	Msg               string
	CarfileBlockCount int
	DoneBlockCount    int
	CarfileSize       int64
	DoneSize          int64
	CarfileHash       string
	DiskUsage         float64
	TotalBlockCount   int
}

// RemoveCarfileResultInfo remove carfile result
type RemoveCarfileResultInfo struct {
	BlockCount int
	DiskUsage  float64
}

// CarfileRecordInfo Data info
type CarfileRecordInfo struct {
	CarfileCid          string    `db:"carfile_cid"`
	CarfileHash         string    `db:"carfile_hash"`
	CurReliability      int       `db:"cur_reliability"`
	NeedReliability     int       `db:"need_reliability"`
	TotalSize           int64     `db:"total_size"`
	TotalBlocks         int       `db:"total_blocks"`
	ExpiredTime         time.Time `db:"expired_time"`
	CreateTime          time.Time `db:"created_time"`
	EndTime             time.Time `db:"end_time"`
	CarfileReplicaInfos []*CarfileReplicaInfo
	ResultInfo          *CarfileRecordCacheResult
}

// CarfileReplicaInfo Carfile Replica Info
type CarfileReplicaInfo struct {
	ID          string
	CarfileHash string      `db:"carfile_hash"`
	DeviceID    string      `db:"device_id"`
	Status      CacheStatus `db:"status"`
	DoneSize    int64       `db:"done_size"`
	DoneBlocks  int         `db:"done_blocks"`
	IsCandidate bool        `db:"is_candidate"`
	CreateTime  time.Time   `db:"created_time"`
	EndTime     time.Time   `db:"end_time"`
}

// CacheCarfileInfo Data info
type CacheCarfileInfo struct {
	CarfileCid      string    `db:"carfile_cid"`
	CarfileHash     string    `db:"carfile_hash"`
	CurReliability  int       `db:"cur_reliability"`
	NeedReliability int       `db:"need_reliability"`
	DeviceID        string    `db:"device_id"`
	ExpiredTime     time.Time `db:"expired_time"`
}

type NodeBlockDownloadResult struct {
	// serial number
	SN            int64
	Sign          []byte
	DownloadSpeed int64
	BlockSize     int
	Succeed       bool
	FailedReason  string
	BlockCID      string
}

type DownloadServerAccessAuth struct {
	DeviceID   string
	URL        string
	PrivateKey string
}

type UserBlockDownloadResult struct {
	// serial number
	SN int64
	// user signature
	Sign    []byte
	Succeed bool
}

type DownloadInfoResult struct {
	URL      string
	Sign     string
	SN       int64
	SignTime int64
	TimeOut  int
	Weight   int
	DeviceID string `json:"-"`
}

type CandidateDownloadInfo struct {
	URL   string
	Token string
}

// CacheStatus Cache Status
type CacheStatus int

const (
	// CacheStatusCreate status
	CacheStatusCreate CacheStatus = iota
	// CacheStatusRunning status
	CacheStatusRunning
	// CacheStatusFailed status
	CacheStatusFailed
	// CacheStatusSucceeded status
	CacheStatusSucceeded
)

// CacheError cache error
type CacheError struct {
	CID       string
	Nodes     int
	SkipCount int
	DiskCount int
	Msg       string
	Time      time.Time
	DeviceID  string
}

type NodeAppUpdateInfo struct {
	NodeType    int       `db:"node_type"`
	AppName     string    `db:"app_name"`
	Version     Version   `db:"version"`
	Hash        string    `db:"hash"`
	DownloadURL string    `db:"download_url"`
	UpdateTime  time.Time `db:"update_time"`
}

// NodeCacheState node cache state
type NodeCacheState struct {
	CarfileHash string      `db:"carfile_hash"`
	Status      CacheStatus `db:"status"`
}

// NodeCacheRsp node caches
type NodeCacheRsp struct {
	Caches     []*NodeCacheState
	TotalCount int
}

// CarfileRecordCacheResult cache result
type CarfileRecordCacheResult struct {
	NodeErrs             map[string]string
	ErrMsg               string
	EdgeNodeCacheSummary string
}

type NatType int

const (
	NatTypeUnknow NatType = iota
	// not bihind nat
	NatTypeNo
	NatTypeSymmetric
	NatTypeFullCone
	NatTypeRestricted
	NatTypePortRestricted
)
