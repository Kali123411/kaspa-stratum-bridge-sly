package slyvexstratum

import (
	"context"
	"fmt"
	"time"

	"github.com/slyvexnetwork/slyvexd/app/appmessage"
	"github.com/slyvexnetwork/slyvexd/infrastructure/network/rpcclient"
	"github.com/onemorebsmith/slyvexstratum/src/gostratum"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"github.com/slyvex-core/slyvexd/infrastructure/network/rpcclient"
)

type SlyvexApi struct {
	address       string
	blockWaitTime time.Duration
	logger        *zap.SugaredLogger
	slyvexd       *rpcclient.RPCClient
	connected     bool
}

func NewSlyvexAPI(address string, blockWaitTime time.Duration, logger *zap.SugaredLogger) (*SlyvexApi, error) {
type KaspaAPI struct {
	slyvexd *rpcclient.RPCClient
	logger  *zap.SugaredLogger
}

func NewKaspaAPI(address string, timeout int, logger *zap.SugaredLogger) (*KaspaAPI, error) {
	client, err := rpcclient.NewRPCClient(address)
	if err != nil {
		return nil, err
	}

	return &SlyvexApi{
		address:       address,
		blockWaitTime: blockWaitTime,
		logger:        logger.With(zap.String("component", "slyvexapi:"+address)),
		slyvexd:       client,
		connected:     true,
	}, nil
}

func (sv *SlyvexApi) Start(ctx context.Context, blockCb func()) {
	sv.waitForSync(true)
	go sv.startBlockTemplateListener(ctx, blockCb)
	go sv.startStatsThread(ctx)
}

func (sv *SlyvexApi) startStatsThread(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			sv.logger.Warn("context cancelled, stopping stats thread")
			return
		case <-ticker.C:
			dagResponse, err := sv.slyvexd.GetBlockDAGInfo()
			if err != nil {
				sv.logger.Warn("failed to get network hashrate from slyvex, prom stats will be out of date", zap.Error(err))
				continue
			}
			response, err := sv.slyvexd.EstimateNetworkHashesPerSecond(dagResponse.TipHashes[0], 1000)
			if err != nil {
				sv.logger.Warn("failed to get network hashrate from slyvex, prom stats will be out of date", zap.Error(err))
				continue
			}
			RecordNetworkStats(response.NetworkHashesPerSecond, dagResponse.BlockCount, dagResponse.Difficulty)
		}
	}
}

func (sv *SlyvexApi) reconnect() error {
	if sv.slyvexd != nil {
		return sv.slyvexd.Reconnect()
	}

	client, err := rpcclient.NewRPCClient(sv.address)
	if err != nil {
		return err
	}
	sv.slyvexd = client
	return nil
}

func (s *SlyvexApi) waitForSync(verbose bool) error {
	if verbose {
		s.logger.Info("checking slyvexd sync state")
	}
	for {
		clientInfo, err := s.slyvexd.GetInfo()
		if err != nil {
			return errors.Wrapf(err, "error fetching server info from slyvexd @ %s", s.address)
		}
		if clientInfo.IsSynced {
			break
		}
		s.logger.Warn("Slyvex is not synced, waiting for sync before starting bridge")
		time.Sleep(5 * time.Second)
	}
	if verbose {
		s.logger.Info("slyvexd synced, starting server")
	}
	return nil
}

func (s *SlyvexApi) startBlockTemplateListener(ctx context.Context, blockReadyCb func()) {
	var blockReadyChan chan bool
	restartChannel := true
	ticker := time.NewTicker(s.blockWaitTime)
	for {
		if err := s.waitForSync(false); err != nil {
			s.logger.Error("error checking slyvexd sync state, attempting reconnect: ", err)
			if err := s.reconnect(); err != nil {
				s.logger.Error("error reconnecting to slyvexd, waiting before retry: ", err)
				time.Sleep(5 * time.Second)
			}
			restartChannel = true
		}
		if restartChannel {
			blockReadyChan = make(chan bool)
			err := s.slyvexd.RegisterForNewBlockTemplateNotifications(func(_ *appmessage.NewBlockTemplateNotificationMessage) {
				blockReadyChan <- true
			})
			if err != nil {
				s.logger.Error("fatal: failed to register for block notifications from slyvex")
			} else {
				restartChannel = false
			}
		}
		select {
		case <-ctx.Done():
			s.logger.Warn("context cancelled, stopping block update listener")
			return
		case <-blockReadyChan:
			blockReadyCb()
			ticker.Reset(s.blockWaitTime)
		case <-ticker.C: // timeout, manually check for new blocks
			blockReadyCb()
		}
	}
}

func (sv *SlyvexApi) GetBlockTemplate(
	client *gostratum.StratumContext) (*appmessage.GetBlockTemplateResponseMessage, error) {
	template, err := sv.slyvexd.GetBlockTemplate(client.WalletAddr,
		fmt.Sprintf(`'%s' via onemorebsmith/slyvex-stratum-bridge_%s`, client.RemoteApp, version))
	if err != nil {
		return nil, errors.Wrap(err, "failed fetching new block template from slyvex")
	}
	return template, nil
	return &KaspaAPI{slyvexd: client, logger: logger}, nil
}
