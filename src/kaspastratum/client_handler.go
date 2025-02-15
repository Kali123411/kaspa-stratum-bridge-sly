package slyvexstratum

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kali123411/kaspa-stratum-bridge-sly/src/gostratum"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var bigJobRegex = regexp.MustCompile(".*(BzMiner|IceRiverMiner).*")

const balanceDelay = time.Minute

type clientListener struct {
	logger           *zap.SugaredLogger
	shareHandler     *shareHandler
	clientLock       sync.RWMutex
	clients          map[int32]*gostratum.StratumContext
	lastBalanceCheck time.Time
	clientCounter    int32
	minShareDiff     float64
	extranonceSize   int8
	maxExtranonce    int32
	nextExtranonce   int32
}

func newClientListener(logger *zap.SugaredLogger, shareHandler *shareHandler, minShareDiff float64, extranonceSize int8) *clientListener {
	return &clientListener{
		logger:         logger,
		minShareDiff:   minShareDiff,
		extranonceSize: extranonceSize,
		maxExtranonce:  int32(math.Pow(2, (8*math.Min(float64(extranonceSize), 3))) - 1),
		nextExtranonce: 0,
		clientLock:     sync.RWMutex{},
		shareHandler:   shareHandler,
		clients:        make(map[int32]*gostratum.StratumContext),
	}
}

func (c *clientListener) OnConnect(ctx *gostratum.StratumContext) {
	var extranonce int32

	idx := atomic.AddInt32(&c.clientCounter, 1)
	ctx.Id = idx
	c.clientLock.Lock()
	if c.extranonceSize > 0 {
		extranonce = c.nextExtranonce
		if c.nextExtranonce < c.maxExtranonce {
			c.nextExtranonce++
		} else {
			c.nextExtranonce = 0
			c.logger.Warn("wrapped extranonce! new clients may be duplicating work...")
		}
	}
	c.clients[idx] = ctx
	c.clientLock.Unlock()
	ctx.Logger = ctx.Logger.With(zap.Int("client_id", int(ctx.Id)))

	if c.extranonceSize > 0 {
		ctx.Extranonce = fmt.Sprintf("%0*x", c.extranonceSize*2, extranonce)
	}
	go func() {
		time.Sleep(5 * time.Second) // Allow time for authorization
		c.shareHandler.getCreateStats(ctx) // Create miner stats if missing
	}()
}

func (c *clientListener) OnDisconnect(ctx *gostratum.StratumContext) {
	ctx.Done()
	c.clientLock.Lock()
	c.logger.Info("removing client ", ctx.Id)
	delete(c.clients, ctx.Id)
	c.logger.Info("removed client ", ctx.Id)
	c.clientLock.Unlock()
	RecordDisconnect(ctx)
}

func (c *clientListener) NewBlockAvailable(sapi *SlyvexApi) {
	c.clientLock.Lock()
	addresses := make([]string, 0, len(c.clients))
	for _, cl := range c.clients {
		if !cl.Connected() {
			continue
		}
		go func(client *gostratum.StratumContext) {
			state := GetMiningState(client)
			if client.WalletAddr == "" {
				if time.Since(state.connectTime) > time.Second*20 {
					client.Logger.Warn("client misconfigured, no miner address specified - disconnecting")
					RecordWorkerError(client.WalletAddr, ErrNoMinerAddress)
					client.Disconnect()
				}
				return
			}
			template, err := sapi.GetBlockTemplate(client)
			if err != nil {
				client.Logger.Error(fmt.Sprintf("failed fetching new block template from Slyvex: %s", err))
				client.Disconnect()
				return
			}

			state.bigDiff = CalculateSlyvexDifficulty(template.Block.Header.DifficultyBits)
			header, err := SerializeSlyvexBlockHeader(template.Block)
			if err != nil {
				RecordWorkerError(client.WalletAddr, ErrBadDataFromMiner)
				client.Logger.Error(fmt.Sprintf("failed to serialize block header: %s", err))
				return
			}

			jobId := state.AddJob(template.Block)
			if !state.initialized {
				state.initialized = true
				state.useBigJob = bigJobRegex.MatchString(client.RemoteApp)
				state.stratumDiff = newSlyvexDiff()
				state.stratumDiff.setDiffValue(c.minShareDiff)
				sendClientDiff(client, state)
				c.shareHandler.setClientVardiff(client, c.minShareDiff)
			} else {
				varDiff := c.shareHandler.getClientVardiff(client)
				if varDiff != state.stratumDiff.diffValue && varDiff != 0 {
					client.Logger.Info(fmt.Sprintf("changing diff from %f to %f", state.stratumDiff.diffValue, varDiff))
					state.stratumDiff.setDiffValue(varDiff)
					sendClientDiff(client, state)
					c.shareHandler.startClientVardiff(client)
				}
			}

			jobParams := []any{fmt.Sprintf("%d", jobId)}
			if state.useBigJob {
				jobParams = append(jobParams, GenerateLargeSlyvexJobParams(header, template.Block.Header.Timestamp))
			} else {
				jobParams = append(jobParams, GenerateSlyvexJobHeader(header))
				jobParams = append(jobParams, template.Block.Header.Timestamp)
			}

			if err := client.Send(gostratum.JsonRpcEvent{
				Version: "2.0",
				Method:  "mining.notify",
				Id:      jobId,
				Params:  jobParams,
			}); err != nil {
				RecordWorkerError(client.WalletAddr, ErrFailedSendWork)
				client.Logger.Error(errors.Wrapf(err, "failed sending work packet %d", jobId).Error())
			}

			RecordNewJob(client)
		}(cl)

		if cl.WalletAddr != "" {
			addresses = append(addresses, cl.WalletAddr)
		}
	}
	c.clientLock.Unlock()
}

func sendClientDiff(client *gostratum.StratumContext, state *MiningState) {
	if err := client.Send(gostratum.JsonRpcEvent{
		Version: "2.0",
		Method:  "mining.set_difficulty",
		Params:  []any{state.stratumDiff.diffValue},
	}); err != nil {
		RecordWorkerError(client.WalletAddr, ErrFailedSetDiff)
		client.Logger.Error(errors.Wrap(err, "failed sending difficulty").Error())
		return
	}
	client.Logger.Info(fmt.Sprintf("Setting client diff: %f", state.stratumDiff.diffValue))
}
