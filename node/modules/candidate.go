package modules

import (
	"context"

	"github.com/linguohua/titan/api"
	"github.com/linguohua/titan/node/asset/fetcher"
	"github.com/linguohua/titan/node/asset/storage"
	"github.com/linguohua/titan/node/candidate"
	"github.com/linguohua/titan/node/config"
	"github.com/linguohua/titan/node/modules/dtypes"
	"go.uber.org/fx"
)

type NodeParams struct {
	fx.In

	NodeID        dtypes.NodeID
	InternalIP    dtypes.InternalIP
	storageMgr    *storage.Manager
	BandwidthUP   int64
	BandwidthDown int64
}

// NewBlockFetcherFromIPFS returns a new IPFS block fetcher.
func NewBlockFetcherFromIPFS(cfg *config.CandidateCfg) fetcher.BlockFetcher {
	log.Info("ipfs-api " + cfg.IpfsAPIURL)
	return fetcher.NewIPFS(cfg.IpfsAPIURL, cfg.FetchBlockTimeout, cfg.FetchBlockRetry)
}

// NewTCPServer returns a new TCP server instance.
func NewTCPServer(lc fx.Lifecycle, cfg *config.CandidateCfg, schedulerAPI api.Scheduler) *candidate.TCPServer {
	srv := candidate.NewTCPServer(cfg, schedulerAPI)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go srv.StartTCPServer()
			return nil
		},
		OnStop: srv.Stop,
	})

	return srv
}
