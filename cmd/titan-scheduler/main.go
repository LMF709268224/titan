package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/gbrlsnchs/jwt/v3"
	"github.com/linguohua/titan/api"
	"github.com/linguohua/titan/build"
	lcli "github.com/linguohua/titan/cli"
	cliutil "github.com/linguohua/titan/cli/util"
	"github.com/linguohua/titan/lib/titanlog"
	"github.com/linguohua/titan/lib/ulimit"
	"github.com/linguohua/titan/metrics"
	"github.com/linguohua/titan/node/repo"
	"github.com/linguohua/titan/node/scheduler"
	"github.com/linguohua/titan/node/scheduler/db/cache"
	"github.com/linguohua/titan/node/scheduler/db/persistent"
	"github.com/linguohua/titan/node/secret"
	"github.com/linguohua/titan/region"
	"github.com/quic-go/quic-go/http3"
	"go.opencensus.io/tag"
	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("main")

const (
	// FlagSchedulerRepo Flag
	FlagSchedulerRepo = "scheduler-repo"

	// FlagSchedulerRepoDeprecation Flag
	FlagSchedulerRepoDeprecation = "schedulerrepo"
)

func main() {
	api.RunningNodeType = api.NodeScheduler

	titanlog.SetupLogLevels()

	local := []*cli.Command{
		runCmd,
		getAPIKeyCmd,
	}

	local = append(local, lcli.SchedulerCmds...)
	local = append(local, lcli.CommonCommands...)

	app := &cli.App{
		Name:                 "titan-scheduler",
		Usage:                "Titan scheduler node",
		Version:              build.UserVersion(),
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    FlagSchedulerRepo,
				Aliases: []string{FlagSchedulerRepoDeprecation},
				EnvVars: []string{"TITAN_SCHEDULER_PATH", "SCHEDULER_PATH"},
				Value:   "~/.titanscheduler", // TODO: Consider XDG_DATA_HOME
				Usage:   fmt.Sprintf("Specify scheduler repo path. flag %s and env TITAN_SCHEDULER_PATH are DEPRECATION, will REMOVE SOON", FlagSchedulerRepoDeprecation),
			},
			&cli.StringFlag{
				Name:    "panic-reports",
				EnvVars: []string{"TITAN_PANIC_REPORT_PATH"},
				Hidden:  true,
				Value:   "~/.titanscheduler", // should follow --repo default
			},
		},

		After: func(c *cli.Context) error {
			if r := recover(); r != nil {
				// Generate report in TITAN_SCHEDULER_PATH and re-raise panic
				build.GeneratePanicReport(c.String("panic-reports"), c.String(FlagSchedulerRepo), c.App.Name)
				log.Panic(r)
			}
			return nil
		},
		Commands: local,
	}
	app.Setup()
	app.Metadata["repoType"] = repo.Scheduler

	if err := app.Run(os.Args); err != nil {
		log.Warnf("%+v", err)
		return
	}
}

type jwtPayload struct {
	Allow []auth.Permission
}

var getAPIKeyCmd = &cli.Command{
	Name:  "get-api-key",
	Usage: "Generate API Key",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "perm",
			Usage: "permission to assign to the token, one of: read, write, sign, admin",
			Value: "",
		},
	},
	Action: func(cctx *cli.Context) error {
		lr, err := openRepo(cctx)
		if err != nil {
			return err
		}
		defer lr.Close() // nolint

		perm := cctx.String("perm")

		p := jwtPayload{}

		idx := 0
		for i, p := range api.AllPermissions {
			if auth.Permission(perm) == p {
				idx = i + 1
			}
		}

		if idx == 0 {
			return fmt.Errorf("--perm flag has to be one of: %s", api.AllPermissions)
		}

		p.Allow = api.AllPermissions[:idx]

		authKey, err := secret.APISecret(lr)
		if err != nil {
			return xerrors.Errorf("setting up api secret: %w", err)
		}

		k, err := jwt.Sign(&p, (*jwt.HMACSHA)(authKey))
		if err != nil {
			return xerrors.Errorf("jwt sign: %w", err)
		}

		fmt.Println(string(k))
		return nil
	},
}

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "Start titan scheduler node",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "listen",
			Usage: "host address and port the scheduler api will listen on",
			Value: "0.0.0.0:3456",
		},
		&cli.StringFlag{
			Name:  "cachedb-url",
			Usage: "cachedb url",
			Value: "127.0.0.1:6379",
		},
		&cli.StringFlag{
			Name:  "geodb-path",
			Usage: "geodb path",
			Value: "../../geoip/geolite2_city/city.mmdb",
		},
		&cli.StringFlag{
			Name:  "persistentdb-url",
			Usage: "persistentdb url",
			Value: "user01:sql001@tcp(127.0.0.1:3306)/test",
		},
		&cli.StringFlag{
			Name:  "server-name",
			Usage: "server uniquely identifies",
		},
		&cli.StringFlag{
			Name:  "area",
			Usage: "area",
			Value: "CN-GD-Shenzhen",
		},
		&cli.StringFlag{
			Required: true,
			Name:     "certificate-path",
			Usage:    "cerfitifcate path, example: --certificate-path=./cert.pem",
			Value:    "",
		},
		&cli.StringFlag{
			Required: true,
			Name:     "private-key-path",
			Usage:    "private key path, example: --private-key-path=./priv.key",
			Value:    "",
		},
		&cli.StringFlag{
			Required: true,
			Name:     "ca-certificate-path",
			Usage:    "root-certificate, example: --ca-certificate-pat=./ca.pem",
			Value:    "",
		},
	},

	Before: func(cctx *cli.Context) error {
		return nil
	},
	Action: func(cctx *cli.Context) error {
		log.Info("Starting titan scheduler node")

		limit, _, err := ulimit.GetLimit()
		switch {
		case err == ulimit.ErrUnsupported:
			log.Errorw("checking file descriptor limit failed", "error", err)
		case err != nil:
			return xerrors.Errorf("checking fd limit: %w", err)
		default:
			if limit < build.EdgeFDLimit {
				return xerrors.Errorf("soft file descriptor limit (ulimit -n) too low, want %d, current %d", build.EdgeFDLimit, limit)
			}
		}

		// Connect to scheduler
		ctx := lcli.ReqContext(cctx)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		cURL := cctx.String("cachedb-url")
		sName := cctx.String("server-name")
		area := cctx.String("area")
		if area == "" {
			log.Panic("area is nil")
		}

		if sName == "" {
			log.Panic("server-name is nil")
		}

		err = cache.NewCacheDB(cURL, cache.TypeRedis(), sName)
		if err != nil {
			log.Panic(err.Error())
		}

		gPath := cctx.String("geodb-path")
		err = region.NewRegion(gPath, region.TypeGeoLite(), area)
		if err != nil {
			log.Panic(err.Error())
		}

		pPath := cctx.String("persistentdb-url")
		err = persistent.NewDB(pPath, persistent.TypeSQL(), sName, area)
		if err != nil {
			log.Panic(err.Error())
		}

		lr, err := openRepo(cctx)
		if err != nil {
			log.Panic(err.Error())
		}

		address := cctx.String("listen")

		addressList := strings.Split(address, ":")
		portStr := addressList[1]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Panic(err.Error())
		}
		schedulerAPI := scheduler.NewLocalScheduleNode(lr, port)
		handler := schedulerHandler(schedulerAPI, true)

		srv := &http.Server{
			Handler: handler,
			BaseContext: func(listener net.Listener) context.Context {
				ctx, _ := tag.New(context.Background(), tag.Upsert(metrics.APIInterface, "titan-edge"))
				return ctx
			},
		}

		udpPacketConn, err := net.ListenPacket("udp", address)
		if err != nil {
			return err
		}
		defer udpPacketConn.Close()

		certificatePath := cctx.String("certificate-path")
		privateKeyPath := cctx.String("private-key-path")
		caCertificatePath := cctx.String("ca-certificate-path")

		httpClient := cliutil.NewHttp3Client(udpPacketConn, caCertificatePath)
		jsonrpc.SetHttp3Client(httpClient)

		go startUDPServer(udpPacketConn, handler, certificatePath, privateKeyPath)

		go func() {
			<-ctx.Done()
			log.Warn("Shutting down...")
			if err := srv.Shutdown(context.TODO()); err != nil {
				log.Errorf("shutting down RPC server failed: %s", err)
			}
			log.Warn("Graceful shutdown successful")
		}()

		nl, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}

		log.Info("titan scheduler listen with:", address)

		return srv.Serve(nl)
	},
}

func openRepo(cctx *cli.Context) (repo.LockedRepo, error) {
	repoPath := cctx.String(FlagSchedulerRepo)
	r, err := repo.NewFS(repoPath)
	if err != nil {
		return nil, err
	}

	ok, err := r.Exists()
	if err != nil {
		return nil, err
	}
	if !ok {
		if err := r.Init(repo.Scheduler); err != nil {
			return nil, err
		}
	}

	lr, err := r.Lock(repo.Scheduler)
	if err != nil {
		return nil, err
	}

	return lr, nil
}

func startUDPServer(conn net.PacketConn, handler http.Handler, certPath, privPath string) error {
	cert, err := tls.LoadX509KeyPair(certPath, privPath)
	if err != nil {
		return err
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

	srv := http3.Server{
		TLSConfig: config,
		Handler:   handler,
	}

	return srv.Serve(conn)
}
