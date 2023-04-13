package node

import (
	"errors"

	"github.com/linguohua/titan/api"
	"github.com/linguohua/titan/node/asset"
	"github.com/linguohua/titan/node/asset/fetcher"
	"github.com/linguohua/titan/node/asset/storage"
	"github.com/linguohua/titan/node/candidate"
	"github.com/linguohua/titan/node/config"
	"github.com/linguohua/titan/node/device"
	"github.com/linguohua/titan/node/modules"
	"github.com/linguohua/titan/node/modules/dtypes"
	"github.com/linguohua/titan/node/repo"
	datasync "github.com/linguohua/titan/node/sync"
	"github.com/linguohua/titan/node/validation"
	"go.uber.org/fx"
	"golang.org/x/time/rate"
	"golang.org/x/xerrors"
)

func Candidate(out *api.Candidate) Option {
	return Options(
		ApplyIf(func(s *Settings) bool { return s.Config },
			Error(errors.New("the Candidate option must be set before Config option")),
		),

		func(s *Settings) error {
			s.nodeType = repo.Candidate
			return nil
		},

		func(s *Settings) error {
			resAPI := &candidate.Candidate{}
			s.invokes[ExtractAPIKey] = fx.Populate(resAPI)
			*out = resAPI
			return nil
		},
	)
}

func ConfigCandidate(c interface{}) Option {
	cfg, ok := c.(*config.CandidateCfg)
	if !ok {
		return Error(xerrors.Errorf("invalid config from repo, got: %T", c))
	}
	log.Info("start to config candidate")

	return Options(
		Override(new(*config.CandidateCfg), cfg),
		Override(new(*device.Device), modules.NewDevice(cfg.BandwidthUp, cfg.BandwidthDown)),
		Override(new(dtypes.NodeMetadataPath), dtypes.NodeMetadataPath(cfg.MetadataPath)),
		Override(new(*storage.Manager), modules.NewNodeStorageManager),
		Override(new(*asset.Manager), modules.NewAssetsManager(cfg.FetchBatch)),
		Override(new(*validation.Validation), modules.NewNodeValidation),
		Override(new(*rate.Limiter), modules.NewRateLimiter),
		Override(new(*asset.Asset), asset.NewAsset),
		Override(new(fetcher.BlockFetcher), modules.NewBlockFetcherFromIPFS),
		Override(new(*datasync.DataSync), modules.NewDataSync),
		Override(new(*candidate.TCPServer), modules.NewTCPServer),
	)
}
