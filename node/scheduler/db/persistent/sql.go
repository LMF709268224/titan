package persistent

import (
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/linguohua/titan/api"
	"golang.org/x/xerrors"
)

// TypeSQL Sql
func TypeSQL() string {
	return "Sql"
}

type sqlDB struct {
	cli *sqlx.DB
	url string
}

const errNodeNotFind = "Not Found"

var (
	deviceBlockTable = "device_blocks_%s"
	dataInfoTable    = "data_info_%s"
	blockInfoTable   = "block_info_%s"
	cacheInfoTable   = "cache_info_%s"
)

// InitSQL init sql
func InitSQL(url string) (DB, error) {
	db := &sqlDB{url: url}
	database, err := sqlx.Open("mysql", url)
	if err != nil {
		return nil, err
	}

	if err := database.Ping(); err != nil {
		return nil, err
	}

	db.cli = database

	err = db.SetAllNodeOffline()
	return db, err
}

// IsNilErr Is NilErr
func (sd sqlDB) IsNilErr(err error) bool {
	return err.Error() == errNodeNotFind
}

func (sd sqlDB) SetNodeInfo(deviceID string, info *NodeInfo) error {
	info.DeviceID = deviceID
	info.ServerName = serverName
	info.CreateTime = time.Now().Format("2006-01-02 15:04:05")

	_, err := sd.GetNodeInfo(deviceID)
	if err != nil {
		if sd.IsNilErr(err) {
			_, err = sd.cli.NamedExec(`INSERT INTO node (device_id, last_time, geo, node_type, is_online, address, server_name, create_time)
                VALUES (:device_id, :last_time, :geo, :node_type, :is_online, :address, :server_name, :create_time)`, info)
		}
		return err
	}

	// update
	_, err = sd.cli.NamedExec(`UPDATE node SET last_time=:last_time,geo=:geo,is_online=:is_online,address=:address,server_name=:server_name  WHERE device_id=:device_id`, info)

	return err
}

func (sd sqlDB) SetAllNodeOffline() error {
	info := &NodeInfo{IsOnline: 0, ServerName: serverName}
	_, err := sd.cli.NamedExec(`UPDATE node SET is_online=:is_online WHERE server_name=:server_name`, info)

	return err
}

func (sd sqlDB) GetNodeInfo(deviceID string) (*NodeInfo, error) {
	info := &NodeInfo{DeviceID: deviceID}

	rows, err := sd.cli.NamedQuery(`SELECT * FROM node WHERE device_id=:device_id`, info)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, xerrors.New(errNodeNotFind)
	}
	rows.Close()

	return info, err
}

func (sd sqlDB) SetValidateResultInfo(info *ValidateResult) error {
	nowTime := time.Now().Format("2006-01-02 15:04:05")

	if ValidateStatus(info.Status) == ValidateStatusCreate || ValidateStatus(info.Status) == ValidateStatusOther {
		info.StratTime = nowTime
		info.ServerName = serverName

		_, err := sd.cli.NamedExec(`INSERT INTO validate_result (round_id, device_id, validator_id, status, strat_time, server_name, msg)
                VALUES (:round_id, :device_id, :validator_id, :status, :strat_time, :server_name, :msg)`, info)
		return err

	} else if ValidateStatus(info.Status) > ValidateStatusCreate {
		info.EndTime = nowTime

		_, err := sd.cli.NamedExec(`UPDATE validate_result SET end_time=:end_time,status=:status,msg=:msg  WHERE device_id=:device_id AND round_id=:round_id`, info)

		return err
	}

	return xerrors.Errorf("SetValidateResultInfo err deviceid:%s ,status:%d, roundID:%s, serverName:%s", info.DeviceID, info.Status, info.RoundID, info.ServerName)
}

func (sd sqlDB) SetNodeToValidateErrorList(sID, deviceID string) error {
	_, err := sd.cli.NamedExec(`INSERT INTO validate_err (round_id, device_id)
                VALUES (:round_id, :device_id)`, map[string]interface{}{
		"round_id":  sID,
		"device_id": deviceID,
	})
	return err
}

// func (sd sqlDB) RemoveBlockInfo(deviceID, cid string) error {
// 	info := NodeBlocks{
// 		CID: cid,
// 	}

// 	cmd := fmt.Sprintf(`DELETE FROM %s WHERE cid=:cid`, fmt.Sprintf(deviceBlockTable, deviceID))
// 	_, err := sd.cli.NamedExec(cmd, info)
// 	return err
// }

// func (sd sqlDB) SetCarfileInfo(deviceID, cid, carfileID, cacheID string) error {
// 	info := NodeBlock{
// 		TableName: fmt.Sprintf(deviceBlockTable, deviceID),
// 		CID:       cid,
// 		CarfileID: carfileID,
// 		CacheID:   cacheID,
// 	}

// 	cmd := fmt.Sprintf(`UPDATE %s SET carfile_id=:carfile_id,cache_id=:cache_id WHERE cid=:cid`, info.TableName)
// 	_, err := sd.cli.NamedExec(cmd, info)
// 	return err
// }

// func (sd sqlDB) SetBlockInfo(deviceID, cid, fid, carfileID, cacheID string, isUpdate bool) error {
// 	tableName := fmt.Sprintf(deviceBlockTable, deviceID)

// 	info := NodeBlocks{
// 		CID:       cid,
// 		FID:       fid,
// 		CarfileID: carfileID,
// 		CacheID:   cacheID,
// 	}

// 	if isUpdate {
// 		cmd := fmt.Sprintf(`UPDATE %s SET fid=:fid,carfile_id=:carfile_id,cache_id=:cache_id WHERE cid=:cid`, tableName)
// 		_, err := sd.cli.NamedExec(cmd, info)

// 		return err
// 	}

// 	schema := fmt.Sprintf(`
// 	CREATE TABLE %s (
// 		cid text,
// 		fid text,
// 		cache_id text,
// 		carfile_id text
// 	);`, tableName)

// 	_, err := sd.cli.Exec(schema)
// 	if err != nil {
// 		if !strings.Contains(err.Error(), "already exists") {
// 			return err
// 		}
// 	}

// 	cmd := fmt.Sprintf(`INSERT INTO %s (cid, fid, cache_id, carfile_id)
// 	VALUES (:cid, :fid, :cache_id, :carfile_id)`, tableName)

// 	_, err = sd.cli.NamedExec(cmd, info)

// 	return err
// }

func (sd sqlDB) GetBlockFidWithCid(deviceID, cid string) (string, error) {
	area := sd.ReplaceArea()

	info := &NodeBlocks{
		CID:      cid,
		DeviceID: deviceID,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE cid=:cid AND device_id=:device_id`, fmt.Sprintf(deviceBlockTable, area))

	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return "", err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return "", err
		}
	} else {
		return "", xerrors.New(errNodeNotFind)
	}
	rows.Close()

	return info.FID, err
}

func (sd sqlDB) GetBlocksFID(deviceID string) (map[string]string, error) {
	area := sd.ReplaceArea()

	m := make(map[string]string)

	info := &NodeBlocks{
		DeviceID: deviceID,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE device_id=:device_id`, fmt.Sprintf(deviceBlockTable, area))
	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		i := &NodeBlocks{}
		err = rows.StructScan(i)
		if err == nil {
			m[i.FID] = i.CID
		}
	}

	return m, nil
}

func (sd sqlDB) GetBlockCidWithFid(deviceID, fid string) (string, error) {
	area := sd.ReplaceArea()

	info := &NodeBlocks{
		FID:      fid,
		DeviceID: deviceID,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE fid=:fid AND device_id=:device_id`, fmt.Sprintf(deviceBlockTable, area))
	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return "", err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return "", err
		}
	} else {
		return "", xerrors.New(errNodeNotFind)
	}
	rows.Close()

	return info.CID, err
}

func (sd sqlDB) GetDeviceBlockNum(deviceID string) (int64, error) {
	area := sd.ReplaceArea()

	var count int64
	cmd := fmt.Sprintf("SELECT count(*) FROM %s WHERE device_id=? ;", fmt.Sprintf(deviceBlockTable, area))
	err := sd.cli.Get(&count, cmd, deviceID)

	return count, err
}

func (sd sqlDB) SetDataInfo(info *DataInfo) error {
	area := sd.ReplaceArea()

	tableName := fmt.Sprintf(dataInfoTable, area)

	_, err := sd.GetDataInfo(info.CID)
	if err != nil {
		if sd.IsNilErr(err) {
			cmd := fmt.Sprintf("INSERT INTO %s (cid, cache_ids, status, need_reliability, total_blocks) VALUES (:cid, :cache_ids, :status, :need_reliability, :total_blocks)", tableName)
			_, err = sd.cli.NamedExec(cmd, info)
		}
		return err
	}

	// update
	cmd := fmt.Sprintf("UPDATE %s SET cache_ids=:cache_ids,status=:status,total_size=:total_size,reliability=:reliability,cache_time=:cache_time,root_cache_id=:root_cache_id,total_blocks=:total_blocks WHERE cid=:cid", tableName)
	_, err = sd.cli.NamedExec(cmd, info)

	return err
}

func (sd sqlDB) GetDataInfo(cid string) (*DataInfo, error) {
	area := sd.ReplaceArea()

	info := &DataInfo{CID: cid}

	cmd := fmt.Sprintf("SELECT * FROM %s WHERE cid=:cid", fmt.Sprintf(dataInfoTable, area))

	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, xerrors.New(errNodeNotFind)
	}
	rows.Close()

	return info, err
}

func (sd sqlDB) GetDataInfos() ([]*DataInfo, error) {
	area := sd.ReplaceArea()

	list := make([]*DataInfo, 0)

	cmd := fmt.Sprintf("SELECT * FROM %s", fmt.Sprintf(dataInfoTable, area))
	rows, err := sd.cli.Queryx(cmd)
	if err != nil {
		return list, err
	}
	defer rows.Close()

	for rows.Next() {
		info := &DataInfo{}
		err := rows.StructScan(info)
		if err == nil {
			list = append(list, info)
		}
	}

	return list, err
}

// func (sd sqlDB) CreateCacheInfo(area string, cacheID string) error {
// 	schema := fmt.Sprintf(`
// 	CREATE TABLE %s (
// 		cid text,
// 		device_id text,
// 		status integer,
// 		total_size integer,
// 		reliability integer
// 	);`, cacheID)

// 	_, err := sd.cli.Exec(schema)
// 	if err != nil {
// 		if !strings.Contains(err.Error(), "already exists") {
// 			return err
// 		}
// 	}

// 	return nil
// }

// func (sd sqlDB) SetCacheInfos(area string, infos []*BlockInfo, isUpdate bool) error {
// 	area = sd.ReplaceArea(area)
// 	tableName := fmt.Sprintf(blockInfoTable, area)

// 	tx := sd.cli.MustBegin()

// 	if isUpdate {
// 		for _, info := range infos {
// 			cmd := fmt.Sprintf(`UPDATE %s SET status=?,size=?,reliability=?,device_id=? WHERE id=:id`, tableName)
// 			tx.MustExec(cmd, info.Status, info.Size, info.Reliability, info.DeviceID, info.ID)
// 		}
// 	} else {
// 		for _, info := range infos {
// 			cmd := fmt.Sprintf(`INSERT INTO %s (cache_id, cid, device_id, status, size, reliability) VALUES (?, ?, ?, ?, ?)`, tableName)
// 			tx.MustExec(cmd, info.CacheID, info.CID, info.DeviceID, info.Status, info.Size, info.Reliability)
// 		}
// 	}

// 	return tx.Commit()
// }

func (sd sqlDB) SetCacheInfo(info *CacheInfo) error {
	area := sd.ReplaceArea()
	tableName := fmt.Sprintf(cacheInfoTable, area)

	oldInfo, err := sd.GetCacheInfo(info.CacheID, info.CarfileID)
	if err != nil {
		return err
	}

	if oldInfo != nil {
		info.ID = oldInfo.ID
		cmd := fmt.Sprintf(`UPDATE %s SET done_size=:done_size,done_blocks=:done_blocks,reliability=:reliability,status=:status WHERE id=:id `, tableName)
		_, err = sd.cli.NamedExec(cmd, info)
	} else {
		cmd := fmt.Sprintf("INSERT INTO %s (carfile_id, cache_id, status) VALUES (:carfile_id, :cache_id, :status)", tableName)
		_, err = sd.cli.NamedExec(cmd, info)
	}

	return err
}

func (sd sqlDB) GetCacheInfo(cacheID, carFileCid string) (*CacheInfo, error) {
	area := sd.ReplaceArea()

	info := &CacheInfo{
		CacheID:   cacheID,
		CarfileID: carFileCid,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE carfile_id=:carfile_id AND cache_id=:cache_id`, fmt.Sprintf(cacheInfoTable, area))

	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, nil
	}
	rows.Close()

	return info, err
}

func (sd sqlDB) SetBlockInfos(infos []*BlockInfo) error {
	area := sd.ReplaceArea()
	tableName := fmt.Sprintf(blockInfoTable, area)

	tx := sd.cli.MustBegin()

	for _, info := range infos {
		if info.ID != 0 {
			cmd := fmt.Sprintf(`UPDATE %s SET status=?,size=?,reliability=?,device_id=? WHERE id=?`, tableName)
			tx.MustExec(cmd, info.Status, info.Size, info.Reliability, info.DeviceID, info.ID)
		} else {
			cmd := fmt.Sprintf(`INSERT INTO %s (cache_id, cid, device_id, status, size, reliability) VALUES (?, ?, ?, ?, ?, ?)`, tableName)
			tx.MustExec(cmd, info.CacheID, info.CID, info.DeviceID, info.Status, info.Size, info.Reliability)
		}

		// if fid != "" {
		// 	cmd1 := fmt.Sprintf(`INSERT INTO %s (cid, fid, cache_id, carfile_id, device_id) VALUES (?, ?, ?, ?, ?)`, fmt.Sprintf(deviceBlockTable, area))
		// 	tx.MustExec(cmd1, info.CID, fid, info.CacheID, carfileCid, info.DeviceID)
		// }
	}

	return tx.Commit()
}

func (sd sqlDB) SetBlockInfo(info *BlockInfo, carfileCid, fid string, isUpdate bool) error {
	area := sd.ReplaceArea()
	tableName := fmt.Sprintf(blockInfoTable, area)

	tx := sd.cli.MustBegin()

	if isUpdate {
		cmd := fmt.Sprintf(`UPDATE %s SET status=?,size=?,reliability=?,device_id=? WHERE id=?`, tableName)
		tx.MustExec(cmd, info.Status, info.Size, info.Reliability, info.DeviceID, info.ID)
	} else {
		cmd := fmt.Sprintf(`INSERT INTO %s (cache_id, cid, device_id, status, size, reliability) VALUES (?, ?, ?, ?, ?, ?)`, tableName)
		tx.MustExec(cmd, info.CacheID, info.CID, info.DeviceID, info.Status, info.Size, info.Reliability)
	}

	if fid != "" {
		cmd1 := fmt.Sprintf(`INSERT INTO %s (cid, fid, cache_id, carfile_id, device_id) VALUES (?, ?, ?, ?, ?)`, fmt.Sprintf(deviceBlockTable, area))
		tx.MustExec(cmd1, info.CID, fid, info.CacheID, carfileCid, info.DeviceID)
	}

	return tx.Commit()
}

func (sd sqlDB) GetBlockInfo(cacheID, cid, deviceID string) (*BlockInfo, error) {
	area := sd.ReplaceArea()

	info := &BlockInfo{
		CacheID:  cacheID,
		CID:      cid,
		DeviceID: deviceID,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE cache_id=:cache_id AND cid=:cid AND device_id=:device_id`, fmt.Sprintf(blockInfoTable, area))

	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, nil
	}
	rows.Close()

	return info, err
}

func (sd sqlDB) GetUndoneBlocks(cacheID string) (map[string]int, error) {
	area := sd.ReplaceArea()

	list := make(map[string]int, 0)

	i := &BlockInfo{CacheID: cacheID, Status: 3}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE cache_id=:cache_id AND status<:status`, fmt.Sprintf(blockInfoTable, area))
	rows, err := sd.cli.NamedQuery(cmd, i)
	if err != nil {
		return list, err
	}
	defer rows.Close()

	for rows.Next() {
		info := &BlockInfo{}
		err := rows.StructScan(info)
		if err == nil {
			list[info.CID] = info.ID
			// list = append(list, info.CID)
		}
	}

	return list, err
}

func (sd sqlDB) HaveBlocks(cacheID string, status int) (bool, error) {
	area := sd.ReplaceArea()

	info := &BlockInfo{
		CacheID: cacheID,
		Status:  status,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE cache_id=:cache_id AND status=:status`, fmt.Sprintf(blockInfoTable, area))
	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if rows.Next() {
		return true, nil
	}

	return false, nil
}

// func (sd sqlDB) GetCacheInfos(area, cacheID string) ([]*BlockInfo, error) {
// 	area = sd.ReplaceArea(area)

// 	list := make([]*BlockInfo, 0)

// 	i := &BlockInfo{}
// 	i.CacheID = cacheID

// 	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE cache_id=:cache_id`, fmt.Sprintf(blockInfoTable, area))
// 	rows, err := sd.cli.NamedQuery(cmd, i)
// 	if err != nil {
// 		return list, err
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		info := &BlockInfo{}
// 		err := rows.StructScan(info)
// 		if err == nil {
// 			list = append(list, info)
// 		}
// 	}

// 	return list, err
// }

func (sd sqlDB) BindRegisterInfo(secret, deviceID string, nodeType api.NodeType) error {
	info := api.NodeRegisterInfo{
		Secret:     secret,
		DeviceID:   deviceID,
		NodeType:   int(nodeType),
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	_, err := sd.cli.NamedExec(`INSERT INTO register (device_id, secret, create_time, node_type)
	VALUES (:device_id, :secret, :create_time, :node_type)`, info)

	return err
}

func (sd sqlDB) GetRegisterInfo(deviceID string) (*api.NodeRegisterInfo, error) {
	info := &api.NodeRegisterInfo{
		DeviceID: deviceID,
	}

	rows, err := sd.cli.NamedQuery(`SELECT * FROM register WHERE device_id=:device_id`, info)
	if err != nil {
		return info, err
	}
	if rows.Next() {
		err = rows.StructScan(info)
		if err != nil {
			return info, err
		}
	} else {
		return info, xerrors.New(errNodeNotFind)
	}
	rows.Close()

	return info, err
}

// func (sd sqlDB) RemoveNodeWithCacheList(deviceID, cid string) error {
// 	info := BlockNodes{
// 		DeviceID: deviceID,
// 	}

// 	cmd := fmt.Sprintf(`DELETE FROM %s WHERE device_id=:device_id`, fmt.Sprintf(blockDevicesTable, cid))
// 	_, err := sd.cli.NamedExec(cmd, info)
// 	return err
// }

// func (sd sqlDB) SetNodeToCacheList(deviceID, cid string) error {
// 	tableName := fmt.Sprintf(blockDevicesTable, cid)
// 	if sd.IsNodeInCacheList(deviceID, cid) {
// 		return nil
// 	}

// 	info := BlockNodes{
// 		DeviceID: deviceID,
// 	}

// 	schema := fmt.Sprintf(`
// 	CREATE TABLE %s (
// 		device_id text
// 	);`, tableName)

// 	_, err := sd.cli.Exec(schema)
// 	if err != nil {
// 		if !strings.Contains(err.Error(), "already exists") {
// 			return err
// 		}
// 	}

// 	cmd := fmt.Sprintf(`INSERT INTO %s (device_id)
// 	VALUES (:device_id)`, tableName)

// 	_, err = sd.cli.NamedExec(cmd, info)

// 	return err
// }

// func (sd sqlDB) IsNodeInCacheList(deviceID, cid string) bool {
// 	info := &BlockNodes{
// 		DeviceID: deviceID,
// 	}

// 	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE device_id=:device_id`, fmt.Sprintf(blockDevicesTable, cid))

// 	rows, err := sd.cli.NamedQuery(cmd, info)
// 	if err != nil {
// 		return false
// 	}
// 	if rows.Next() {
// 		err = rows.StructScan(info)
// 		if err != nil {
// 			return false
// 		}
// 	} else {
// 		return false
// 	}
// 	rows.Close()

// 	return true
// }

func (sd sqlDB) GetNodesWithCacheList(cid string) ([]string, error) {
	area := sd.ReplaceArea()

	m := make([]string, 0)

	info := &NodeBlocks{
		CID: cid,
	}

	cmd := fmt.Sprintf(`SELECT * FROM %s WHERE cid=:cid`, fmt.Sprintf(deviceBlockTable, area))

	rows, err := sd.cli.NamedQuery(cmd, info)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		i := &NodeBlocks{}
		err = rows.StructScan(i)
		if err == nil {
			m = append(m, i.DeviceID)
		}
	}

	return m, nil
}

func (sd sqlDB) DeleteBlockInfo(deviceID, cid string) error {
	area := sd.ReplaceArea()

	// info1 := NodeBlocks{
	// 	CID:      cid,
	// 	DeviceID: deviceID,
	// }

	// info2 := BlockNodes{
	// 	DeviceID: deviceID,
	// 	CID:      cid,
	// }

	tx := sd.cli.MustBegin()
	cmd1 := fmt.Sprintf(`DELETE FROM %s WHERE cid=? AND device_id=?`, fmt.Sprintf(deviceBlockTable, area))
	tx.MustExec(cmd1, cid, deviceID)

	// cmd2 := fmt.Sprintf(`DELETE FROM %s WHERE cid=? AND device_id=?`, fmt.Sprintf(blockDevicesTable, area))
	// tx.MustExec(cmd2, cid, deviceID)

	return tx.Commit()
}

// func (sd sqlDB) AddBlockInfo(area, deviceID, cid, fid, carfileID, cacheID string) error {
// 	area = sd.ReplaceArea(area)

// 	info1 := NodeBlocks{
// 		DeviceID:  deviceID,
// 		CID:       cid,
// 		FID:       fid,
// 		CarfileID: carfileID,
// 		CacheID:   cacheID,
// 	}

// 	// info2 := BlockNodes{
// 	// 	DeviceID: deviceID,
// 	// 	CID:      cid,
// 	// }

// 	tx := sd.cli.MustBegin()

// 	cmd1 := fmt.Sprintf(`INSERT INTO %s (cid, fid, cache_id, carfile_id, device_id) VALUES (?, ?, ?, ?, ?)`, fmt.Sprintf(deviceBlockTable, area))
// 	tx.MustExec(cmd1, info1.CID, info1.FID, info1.CacheID, info1.CarfileID, info1.DeviceID)
// 	// cmd2 := fmt.Sprintf(`INSERT INTO %s (cid,  device_id) VALUES (?, ?)`, fmt.Sprintf(blockDevicesTable, area))
// 	// tx.MustExec(cmd2, info2.CID, info2.DeviceID)

// 	return tx.Commit()
// }

func (sd sqlDB) ReplaceArea() string {
	str := strings.ToLower(serverArea)
	str = strings.Replace(str, "-", "_", -1)

	return str
}
