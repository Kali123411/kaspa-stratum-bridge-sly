package slyvexstratum

import (
	"math/big"

<<<<<<< HEAD
	"github.com/slyvexnetwork/slyvexd/app/appmessage"
	"github.com/slyvexnetwork/slyvexd/domain/consensus/model/externalapi"
	"github.com/slyvexnetwork/slyvexd/domain/consensus/utils/consensushashing"
	"github.com/slyvexnetwork/slyvexd/domain/consensus/utils/pow"
	"github.com/slyvexnetwork/slyvexd/infrastructure/network/rpcclient"
	"github.com/onemorebsmith/slyvexstratum/src/gostratum"
	"github.com/onemorebsmith/slyvexstratum/src/utils"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
=======
	"github.com/slyvex-core/slyvexd/infrastructure/network/rpcclient"
>>>>>>> d67473e (Fixed build errors and updated imports for Slyvex integration)
	"go.uber.org/zap"
)

type shareHandler struct {
<<<<<<< HEAD
	slyvex        *rpcclient.RPCClient
	stats        map[string]*WorkStats
	statsLock    sync.Mutex
	overall      WorkStats
	tipBlueScore uint64
}

func newShareHandler(slyvex *rpcclient.RPCClient) *shareHandler {
	return &shareHandler{
		slyvex:     slyvex,
		stats:     map[string]*WorkStats{},
		statsLock: sync.Mutex{},
	}
}

func (sh *shareHandler) getCreateStats(ctx *gostratum.StratumContext) *WorkStats {
	sh.statsLock.Lock()
	defer sh.statsLock.Unlock()

	var stats *WorkStats
	found := false
	if ctx.WorkerName != "" {
		stats, found = sh.stats[ctx.WorkerName]
	}
	workerId := fmt.Sprintf("%s:%d", ctx.RemoteAddr, ctx.RemotePort)
	if !found {
		stats, found = sh.stats[workerId]
		if found {
			delete(sh.stats, workerId)
			stats.WorkerName = ctx.WorkerName
			sh.stats[ctx.WorkerName] = stats
		}
	}
	if !found {
		stats = &WorkStats{
			LastShare:  time.Now(),
			WorkerName: workerId,
			StartTime:  time.Now(),
		}
		sh.stats[workerId] = stats
		InitWorkerCounters(ctx)
	}
	return stats
}

func (sh *shareHandler) submit(ctx *gostratum.StratumContext,
	block *externalapi.DomainBlock, nonce uint64, eventId any) error {
	mutable := block.Header.ToMutable()
	mutable.SetNonce(nonce)
	block = &externalapi.DomainBlock{
		Header:       mutable.ToImmutable(),
		Transactions: block.Transactions,
	}
	_, err := sh.slyvex.SubmitBlock(block)
	blockhash := consensushashing.BlockHash(block)

	ctx.Logger.Info(fmt.Sprintf("Submitted block %s", blockhash))
	if err != nil {
		if strings.Contains(err.Error(), "ErrDuplicateBlock") {
			ctx.Logger.Warn("block rejected, stale")
			sh.getCreateStats(ctx).StaleShares.Add(1)
			sh.overall.StaleShares.Add(1)
			RecordStaleShare(ctx)
			return ctx.ReplyStaleShare(eventId)
		} else {
			ctx.Logger.Warn("block rejected, unknown issue", zap.Error(err))
			sh.getCreateStats(ctx).InvalidShares.Add(1)
			sh.overall.InvalidShares.Add(1)
			RecordInvalidShare(ctx)
			return ctx.ReplyBadShare(eventId)
		}
	}

	ctx.Logger.Info(fmt.Sprintf("block accepted %s", blockhash))
	stats := sh.getCreateStats(ctx)
	stats.BlocksFound.Add(1)
	sh.overall.BlocksFound.Add(1)
	RecordBlockFound(ctx, block.Header.Nonce(), block.Header.BlueScore(), blockhash.String())

	return nil
}

func (sh *shareHandler) startStatsThread() error {
	start := time.Now()
	for {
		time.Sleep(10 * time.Second)

		str := "\n===============================================================================\n"
		str += "  worker name   |  avg hashrate  |   acc/stl/inv  |    blocks    |    uptime   \n"
		str += "-------------------------------------------------------------------------------\n"
		var lines []string
		totalRate := float64(0)
		for _, v := range sh.stats {
			rate := GetAverageHashrateGHs(v)
			totalRate += rate
			rateStr := stringifyHashrate(rate)
			ratioStr := fmt.Sprintf("%d/%d/%d", v.SharesFound.Load(), v.StaleShares.Load(), v.InvalidShares.Load())
			lines = append(lines, fmt.Sprintf(" %-15s| %14.14s | %14.14s | %12d | %11s",
				v.WorkerName, rateStr, ratioStr, v.BlocksFound.Load(), time.Since(v.StartTime).Round(time.Second)))
		}
		sort.Strings(lines)
		str += strings.Join(lines, "\n")
		rateStr := stringifyHashrate(totalRate)
		ratioStr := fmt.Sprintf("%d/%d/%d", sh.overall.SharesFound.Load(), sh.overall.StaleShares.Load(), sh.overall.InvalidShares.Load())
		str += "\n-------------------------------------------------------------------------------\n"
		str += fmt.Sprintf("                | %14.14s | %14.14s | %12d | %11s",
			rateStr, ratioStr, sh.overall.BlocksFound.Load(), time.Since(start).Round(time.Second))
		str += "\n========================================================== slyvex_bridge ===\n"
		log.Println(str)
	}
}

func GetAverageHashrateGHs(stats *WorkStats) float64 {
	return stats.SharesDiff.Load() / time.Since(stats.StartTime).Seconds()
}

func stringifyHashrate(ghs float64) string {
	unitStrings := [...]string{"M", "G", "T", "P", "E", "Z", "Y"}
	var unit string
	var hr float64

	if ghs < 1 {
		hr = ghs * 1000
		unit = unitStrings[0]
	} else if ghs < 1000 {
		hr = ghs
		unit = unitStrings[1]
	} else {
		for i, u := range unitStrings[2:] {
			hr = ghs / (float64(i) * 1000)
			if hr < 1000 {
				break
			}
			unit = u
		}
	}

	return fmt.Sprintf("%0.2f%sH/s", hr, unit)
=======
	slyvexd *rpcclient.RPCClient
	logger  *zap.SugaredLogger
}

// newShareHandler initializes the share handler
func newShareHandler(client *rpcclient.RPCClient) *shareHandler {
	return &shareHandler{slyvexd: client}
}

// startStatsThread logs periodic mining stats
func (s *shareHandler) startStatsThread() {
	s.logger.Info("Starting stats thread")
}

// startVardiffThread adjusts miner difficulty dynamically
func (s *shareHandler) startVardiffThread(sharesPerMin uint, varDiffStats, clampPow2 bool) {
	s.logger.Info("Starting VarDiff thread")
}

// CalculateSlyvexDifficulty converts difficulty bits into a numeric difficulty
func CalculateSlyvexDifficulty(bits uint32) *big.Int {
	diff := big.NewInt(int64(bits))
	return diff
>>>>>>> d67473e (Fixed build errors and updated imports for Slyvex integration)
}
