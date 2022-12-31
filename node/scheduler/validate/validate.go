package validate

import (
	"container/list"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/robfig/cron"
	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log/v2"
	"github.com/linguohua/titan/node/scheduler/node"

	"github.com/linguohua/titan/api"
	"github.com/linguohua/titan/node/helper"
	"github.com/linguohua/titan/node/scheduler/db/cache"
	"github.com/linguohua/titan/node/scheduler/db/persistent"
)

const (
	errMsgTimeOut = "time out"
	errMsgCancel  = "verification canceled due to download"
)

type validateState int

const (
	validationNotStarted validateState = iota
	validationStarting
)

var log = logging.Logger("scheduler/validate")

type Validate struct {
	ctx  context.Context
	seed int64

	// validate round number
	// previous round
	preRoundId int64
	// current round number
	curRoundId int64

	duration int

	// validate start-up time interval (minute)
	interval int

	// fid is maximum value of each device storage record
	// key is device id
	// value is fid
	maxFidMap sync.Map

	// temporary storage of call back message
	resultQueue *list.List
	// heartbeat of call back
	resultChannel chan bool

	nodeManager *node.Manager

	// timer
	crontab *cron.Cron

	// validate state
	validateState validateState

	// validate switch
	enable bool
}

func NewValidate(manager *node.Manager, enable bool) *Validate {
	e := &Validate{
		ctx:           context.Background(),
		duration:      10,
		interval:      5,
		resultQueue:   list.New(),
		resultChannel: make(chan bool, 1),
		nodeManager:   manager,
		crontab:       cron.New(),
		validateState: validationNotStarted,
		enable:        enable,
	}

	e.initValidateTask()

	return e
}

// validation task scheduled initialization
func (v *Validate) initValidateTask() {
	spec := fmt.Sprintf("0 */%d * * * *", v.interval)
	err := v.crontab.AddFunc(spec, func() {
		err := v.startValidate()
		if err != nil {
			log.Errorf("verification failed to open")
		}
	})

	if err != nil {
		log.Panicf(err.Error())
	}

	v.crontab.Start()

	// wait call back message
	go v.initCallbackTask()
}

func (v *Validate) startValidate() error {
	log.Info("=======>> start validate <<=======")

	pre, cur, err := cache.GetDB().GetPreviousAndCurrentRoundId()
	if err != nil {
		return err
	}
	v.preRoundId = pre
	v.curRoundId = cur
	v.seed = time.Now().UnixNano()

	// before opening validation
	// check the last round of verification
	err = v.checkValidateTimeOut()
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	if !v.enable {
		v.validateState = validationNotStarted
		return nil
	}

	err = v.execute()
	if err != nil {
		v.validateState = validationNotStarted
		log.Errorf(err.Error())
		return err
	}
	return nil
}

func (v *Validate) checkValidateTimeOut() error {
	deviceIDs, err := cache.GetDB().GetNodesWithVerifyingList()
	if err != nil {
		return err
	}

	if deviceIDs != nil && len(deviceIDs) > 0 {
		log.Infof("checkValidateTimeOut list:%v", deviceIDs)

		for _, deviceID := range deviceIDs {
			di := deviceID
			go func() {
				err := v.UpdateFailValidateResult(v.preRoundId, di, errMsgTimeOut, persistent.ValidateStatusTimeOut)
				if err != nil {
					log.Errorf(err.Error())
				}
			}()
		}
	}

	return nil
}

func (v *Validate) execute() error {
	v.validateState = validationStarting

	err := cache.GetDB().RemoveVerifyingList()
	if err != nil {
		return err
	}

	validatorList, err := cache.GetDB().GetValidatorsWithList()
	if err != nil {
		return err
	}

	// no successful election
	if validatorList == nil || len(validatorList) == 0 {
		log.Warn("validator list is null")
		return nil
	}

	log.Debug("validator is", validatorList)

	validatorMap, err := v.validateMapping(validatorList)
	if err != nil {
		return err
	}

	for validatorID, validatedList := range validatorMap {
		go func(vId string, list []tmpDeviceMeta) {
			log.Debugf("validator id : %s, validated list : %v", vId, list)
			req := v.assemblyValidateReqAndStore(vId, list)

			validator := v.nodeManager.GetCandidateNode(vId)
			if validator == nil {
				log.Errorf("validator [%s] is null", vId)
				return
			}

			err = validator.GetAPI().ValidateBlocks(v.ctx, req)
			if err != nil {
				log.Errorf(err.Error())
				return
			}
			log.Debugf("validator id : %s, send success", vId)

		}(validatorID, validatedList)
	}
	return nil
}

type tmpDeviceMeta struct {
	deviceId string
	nodeType api.NodeType
	addr     string
}

// verified edge node are allocated to verifiers one by one
func (v *Validate) validateMapping(validatorList []string) (map[string][]tmpDeviceMeta, error) {
	result := make(map[string][]tmpDeviceMeta)
	// edges := v.nodeManager.GetAllEdge()

	v.nodeManager.EdgeNodeMap.Range(func(key, value interface{}) bool {
		edgeNode := value.(*node.EdgeNode)
		var tn tmpDeviceMeta
		tn.nodeType = api.NodeEdge
		tn.deviceId = edgeNode.DeviceId
		tn.addr = edgeNode.Node.GetAddress()

		validatorID := validatorList[v.generatorForRandomNumber(0, len(validatorList))]

		if validated, ok := result[validatorID]; ok {
			validated = append(validated, tn)
			result[validatorID] = validated
		} else {
			vd := make([]tmpDeviceMeta, 0)
			vd = append(vd, tn)
			result[validatorID] = vd
		}

		return true
	})

	// candidates := v.nodeManager.GetAllCandidate()

	v.nodeManager.CandidateNodeMap.Range(func(key, value interface{}) bool {
		candidateNode := value.(*node.CandidateNode)
		var tn tmpDeviceMeta
		tn.deviceId = candidateNode.DeviceId
		tn.nodeType = api.NodeCandidate
		tn.addr = candidateNode.Node.GetAddress()

		validatorID := validatorList[v.generatorForRandomNumber(0, len(validatorList))]
		if validatorID == candidateNode.DeviceId {
			return true
		}

		if validated, ok := result[validatorID]; ok {
			validated = append(validated, tn)
			result[validatorID] = validated
		} else {
			vd := make([]tmpDeviceMeta, 0)
			vd = append(vd, tn)
			result[validatorID] = vd
		}

		return true
	})

	if len(result) == 0 {
		return nil, fmt.Errorf("%s", "edge node and candidate node are empty")
	}

	return result, nil
}

func (v *Validate) generatorForRandomNumber(start, end int) int {
	max := end - start
	if max <= 0 {
		return start
	}
	rand.Seed(time.Now().UnixNano())
	y := rand.Intn(max)
	return start + y
}

func (v *Validate) assemblyValidateReqAndStore(validatorID string, list []tmpDeviceMeta) []api.ReqValidate {
	req := make([]api.ReqValidate, 0)

	for _, device := range list {
		// count device cid number
		num, err := persistent.GetDB().CountCidOfDevice(device.deviceId)
		if err != nil {
			log.Warnf("failed to count cid from device : %s", device.deviceId)
			continue
		}

		// there is no cached cid in the device
		if num <= 0 {
			log.Warnf("no cached cid of device : %s", device.deviceId)
			continue
		}

		maxFid, err := cache.GetDB().GetNodeCacheFid(device.deviceId)
		if err != nil {
			log.Warnf("GetNodeCacheTag err:%s,DeviceId:%s", err.Error(), device.deviceId)
			continue
		}

		v.maxFidMap.Store(device.deviceId, maxFid)

		req = append(req, api.ReqValidate{
			Seed:     v.seed,
			NodeURL:  device.addr,
			Duration: v.duration,
			RoundID:  v.curRoundId,
			NodeType: int(device.nodeType),
			MaxFid:   int(maxFid)},
		)

		err = cache.GetDB().SetNodeToVerifyingList(device.deviceId)
		if err != nil {
			log.Warnf("SetNodeToVerifyingList err:%s, DeviceId:%s", err.Error(), device.deviceId)
			continue
		}

		resultInfo := &persistent.ValidateResult{
			RoundID:     v.curRoundId,
			DeviceID:    device.deviceId,
			ValidatorID: validatorID,
			Status:      persistent.ValidateStatusCreate.Int(),
			StartTime:   time.Now(),
		}

		err = persistent.GetDB().InsertValidateResultInfo(resultInfo)
		if err != nil {
			log.Errorf("InsertValidateResultInfo err:%s, DeviceId:%s", err.Error(), device.deviceId)
			continue
		}
	}

	return req
}

func (v *Validate) getRandNum(max int, r *rand.Rand) int {
	if max > 0 {
		return r.Intn(max)
	}

	return max
}

func (v *Validate) UpdateFailValidateResult(roundId int64, deviceID, msg string, status persistent.ValidateStatus) error {
	resultInfo := &persistent.ValidateResult{RoundID: roundId, DeviceID: deviceID, Msg: msg, Status: status.Int(), EndTime: time.Now()}
	return persistent.GetDB().UpdateFailValidateResultInfo(resultInfo)
}

func (v *Validate) UpdateSuccessValidateResult(validateResults *api.ValidateResults) error {
	resultInfo := &persistent.ValidateResult{
		RoundID:     validateResults.RoundID,
		DeviceID:    validateResults.DeviceID,
		BlockNumber: int64(len(validateResults.Cids)),
		Msg:         "ok",
		Status:      persistent.ValidateStatusSuccess.Int(),
		Bandwidth:   validateResults.Bandwidth,
		Duration:    validateResults.CostTime,
		EndTime:     time.Now(),
	}
	return persistent.GetDB().UpdateSuccessValidateResultInfo(resultInfo)
}

func (v *Validate) initCallbackTask() {
	for {
		select {
		case <-v.resultChannel:
			v.doCallback()
		}
	}
}

func (v *Validate) doCallback() {
	for v.resultQueue.Len() > 0 {
		// take out first element
		element := v.resultQueue.Front()
		if element == nil {
			return
		}
		if validateResults, ok := element.Value.(*api.ValidateResults); ok {
			err := v.handleValidateResult(validateResults)
			if err != nil {
				log.Errorf("deviceId[%s] handle validate result fail : %s", validateResults.DeviceID, err.Error())
			}
		}
		// dequeue
		v.resultQueue.Remove(element)
	}
}

func (v *Validate) PushResultToQueue(validateResults *api.ValidateResults) {
	v.resultQueue.PushBack(validateResults)
	v.resultChannel <- true
}

func (v *Validate) handleValidateResult(validateResults *api.ValidateResults) error {
	if validateResults.RoundID != v.curRoundId {
		return xerrors.Errorf("round id mismatch")
	}

	log.Debugf("validate result : %+v", *validateResults)

	defer func() {
		count, err := cache.GetDB().CountVerifyingNode(v.ctx)
		if err != nil {
			log.Error("CountVerifyingNode fail :", err.Error())
			return
		}
		if count == 0 {
			v.validateState = validationNotStarted
		}
	}()
	defer func() {
		err := cache.GetDB().RemoveNodeWithVerifyingList(validateResults.DeviceID)
		if err != nil {
			log.Errorf("remove edge node [%s] fail : %s", validateResults.DeviceID, err.Error())
			return
		}
	}()

	if validateResults.IsCancel {
		return v.UpdateFailValidateResult(validateResults.RoundID, validateResults.DeviceID, errMsgCancel, persistent.ValidateStatusCancel)
	}

	if validateResults.IsTimeout {
		return v.UpdateFailValidateResult(validateResults.RoundID, validateResults.DeviceID, errMsgTimeOut, persistent.ValidateStatusTimeOut)
	}

	r := rand.New(rand.NewSource(v.seed))
	cidLength := len(validateResults.Cids)

	if cidLength <= 0 || validateResults.RandomCount <= 0 {
		msg := "validate result is null or random count is 0"
		return v.UpdateFailValidateResult(validateResults.RoundID, validateResults.DeviceID, msg, persistent.ValidateStatusFail)
	}

	cacheInfos, err := persistent.GetDB().GetBlocksFID(validateResults.DeviceID)
	if err != nil || len(cacheInfos) <= 0 {
		msg := fmt.Sprintf("failed to query : %s", err.Error())
		return v.UpdateFailValidateResult(validateResults.RoundID, validateResults.DeviceID, msg, persistent.ValidateStatusOther)
	}

	mFValue, _ := v.maxFidMap.Load(validateResults.DeviceID)
	maxFid := mFValue.(int64)

	for index := 0; index < validateResults.RandomCount; index++ {
		fid := v.getRandNum(int(maxFid), r) + 1
		resultCid := validateResults.Cids[index]

		cid := cacheInfos[fid]
		if cid == "" {
			continue
		}

		if !v.compareCid(cid, resultCid) {
			msg := fmt.Sprintf("validate fail; resultCid:%s,cid_db:%s,fid:%d,index:%d", resultCid, cid, fid, index)
			log.Errorf("validate fail deviceID [%s] resultCid:%s,cid_db:%s,fid:%d,index:%d", validateResults.DeviceID, resultCid, cid, fid, index)
			return v.UpdateFailValidateResult(validateResults.RoundID, validateResults.DeviceID, msg, persistent.ValidateStatusFail)
		}
	}

	return v.UpdateSuccessValidateResult(validateResults)
}

func (v *Validate) compareCid(cidStr1, cidStr2 string) bool {
	hash1, err := helper.CIDString2HashString(cidStr1)
	if err != nil {
		return false
	}

	hash2, err := helper.CIDString2HashString(cidStr2)
	if err != nil {
		return false
	}

	return hash1 == hash2
}

func (v *Validate) EnableValidate(enable bool) {
	v.enable = enable
}

func (v *Validate) IsEnable() bool {
	return v.enable
}

func (v *Validate) StartValidateOnceTask() error {
	if v.validateState == validationStarting {
		return fmt.Errorf("validation in progress, cannot start again")
	}

	go func() {
		time.Sleep(time.Duration(v.interval) * time.Minute)
		v.crontab.Start()
	}()

	v.enable = true
	v.crontab.Stop()
	err := v.startValidate()
	if err != nil {
		return err
	}

	return nil
}
