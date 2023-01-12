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
	GetOnlineDeviceIDs(ctx context.Context, nodeType NodeTypeName) ([]string, error)                    //perm:read
	ElectionValidators(ctx context.Context) error                                                       //perm:admin                                                               //perm:admin
	QueryCacheStatWithNode(ctx context.Context, deviceID string) ([]CacheStat, error)                   //perm:read
	QueryCachingBlocksWithNode(ctx context.Context, deviceID string) (CachingBlockList, error)          //perm:read
	CacheCarfile(ctx context.Context, cid string, reliability int, expiredTime time.Time) error         //perm:admin
	RemoveCarfile(ctx context.Context, carfileID string) error                                          //perm:admin
	RemoveCache(ctx context.Context, carfileID, cacheID string) error                                   //perm:admin
	GetCacheData(ctx context.Context, cid string) (CarfileRecordInfo, error)                            //perm:read
	ListCacheDatas(ctx context.Context, page int) (DataListInfo, error)                                 //perm:read
	ShowRunningCacheDatas(ctx context.Context) ([]CarfileRecordInfo, error)                             //perm:read
	RegisterNode(ctx context.Context, nodeType NodeType, count int) ([]NodeRegisterInfo, error)         //perm:admin
	DeleteBlockRecords(ctx context.Context, deviceID string, cids []string) (map[string]string, error)  //perm:admin
	CacheContinue(ctx context.Context, cid, cacheID string) error                                       //perm:admin
	ValidateSwitch(ctx context.Context, open bool) error                                                //perm:admin
	ValidateRunningState(ctx context.Context) (bool, error)                                             //perm:admin
	ValidateStart(ctx context.Context) error                                                            //perm:admin
	ListEvents(ctx context.Context, page int) (EventListInfo, error)                                    //perm:read
	ResetCacheExpiredTime(ctx context.Context, carfileCid, cacheID string, expiredTime time.Time) error //perm:admin
	ReplenishCacheExpiredTime(ctx context.Context, carfileCid, cacheID string, hour int) error          //perm:admin
	NodeQuit(ctx context.Context, device string) error                                                  //perm:admin
	StopCacheTask(ctx context.Context, carfileCid string) error                                         //perm:admin
	RedressDeveiceInfo(ctx context.Context, deviceID string) error                                      //perm:admin

	// call by locator
	LocatorConnect(ctx context.Context, edgePort int, areaID, locatorID, locatorToken string) error //perm:write

	// call by node
	// node send result when user download block complete
	NodeResultForUserDownloadBlock(ctx context.Context, result NodeBlockDownloadResult) error                        //perm:write
	EdgeNodeConnect(ctx context.Context, rpcURL, downloadSrvURL string) error                                        //perm:write
	ValidateBlockResult(ctx context.Context, validateResults ValidateResults) error                                  //perm:write
	CandidateNodeConnect(ctx context.Context, rpcURL, downloadSrvURL string) error                                   //perm:write
	CacheResult(ctx context.Context, resultInfo CacheResultInfo) error                                               //perm:write
	GetCandidateDownloadInfoWithBlocks(ctx context.Context, cids []string) (map[string]CandidateDownloadInfo, error) //perm:write
	GetExternalIP(ctx context.Context) (string, error)                                                               //perm:write
	GetPublicKey(ctx context.Context) (string, error)                                                                //perm:write
	AuthNodeVerify(ctx context.Context, token string) ([]auth.Permission, error)                                     //perm:read
	AuthNodeNew(ctx context.Context, perms []auth.Permission, deviceID, deviceSecret string) ([]byte, error)         //perm:read                                                               //perm:write

	// call by user
	GetDownloadInfosWithCarfile(ctx context.Context, cid, publicKey string) ([]*DownloadInfoResult, error) //perm:read
	GetDevicesInfo(ctx context.Context, deviceID string) (DevicesInfo, error)                              //perm:read
	GetDownloadInfo(ctx context.Context, deviceID string) ([]*BlockDownloadInfo, error)                    //perm:read

	// user send result when user download block complete or failed
	UserDownloadBlockResults(ctx context.Context, results []UserBlockDownloadResult) error //perm:read
}

// DataListInfo Data List Info
type DataListInfo struct {
	Page       int
	TotalPage  int
	Cids       int
	CacheInfos []*CarfileRecordInfo
}

// EventInfo Event Info
type EventInfo struct {
	ID         int
	CID        string    `db:"cid"`
	DeviceID   string    `db:"device_id"`
	User       string    `db:"user"`
	Event      string    `db:"event"`
	Msg        string    `db:"msg"`
	CreateTime time.Time `db:"created_time"`
}

// EventListInfo Event List Info
type EventListInfo struct {
	Page      int
	TotalPage int
	Count     int
	EventList []*EventInfo
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
	Status      CacheStatus
	Msg         string
	TotalBlock  int
	DoneBlocks  int
	TotalSize   int
	DoneSize    int
	CarfileHash string
	DiskUsage   float64
}

// CarfileRecordInfo Data info
type CarfileRecordInfo struct {
	CarfileCid      string    `db:"carfile_cid"`
	CarfileHash     string    `db:"carfile_hash"`
	Reliability     int       `db:"reliability"`
	NeedReliability int       `db:"need_reliability"`
	TotalSize       int       `db:"total_size"`
	TotalBlocks     int       `db:"total_blocks"`
	ExpiredTime     time.Time `db:"expired_time"`
	CreateTime      time.Time `db:"created_time"`
	EndTime         time.Time `db:"end_time"`
	CacheInfos      []CacheTaskInfo
}

// CacheTaskInfo Data Cache info
type CacheTaskInfo struct {
	CarfileHash string      `db:"carfile_hash"`
	DeviceID    string      `db:"device_id"`
	Status      CacheStatus `db:"status"`
	Reliability int         `db:"reliability"`
	DoneSize    int         `db:"done_size"`
	DoneBlocks  int         `db:"done_blocks"`
	CacheCount  int         `db:"cache_count"`
	RootCache   bool        `db:"root_cache"`
	ExpiredTime time.Time   `db:"expired_time"`
	CreateTime  time.Time   `db:"created_time"`
	EndTime     time.Time   `db:"end_time"`
	DataTimeout time.Duration
}

type NodeBlockDownloadResult struct {
	// serial number
	SN int64
	// scheduler signature
	Sign          []byte
	DownloadSpeed int64
	BlockSize     int
	// ClientIP      string
	Result       bool
	FailedReason string
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
	Sign   []byte
	Result bool
}

type DownloadInfoResult struct {
	URL      string
	Sign     string
	SN       int64
	SignTime int64
	TimeOut  int
	DeviceID string `json:"-"`
}

type CandidateDownloadInfo struct {
	URL   string
	Token string
}

// CacheStatus Cache Status
type CacheStatus int

const (
	// CacheStatusUnknown status
	CacheStatusUnknown CacheStatus = iota
	// CacheStatusCreate status
	CacheStatusCreate
	// CacheStatusFail status
	CacheStatusFail
	// CacheStatusSuccess status
	CacheStatusSuccess
	// CacheStatusTimeout status
	CacheStatusTimeout
	// CacheStatusRestore status
	CacheStatusRestore
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
