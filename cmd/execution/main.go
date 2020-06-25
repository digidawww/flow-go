package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/rpc"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	bootstrapFilenames "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
)

func main() {

	var (
		ledgerStorage      *ledger.MTrieStorage
		events             storage.Events
		txResults          storage.TransactionResults
		providerEngine     *provider.Engine
		computationManager *computation.Manager
		ingestionEng       *ingestion.Engine
		rpcConf            rpc.Config
		err                error
		executionState     state.ExecutionState
		triedir            string
		collector          module.ExecutionMetrics
		mTrieCacheSize     uint32
	)

	cmd.FlowNode(flow.RoleExecution.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 1000, "cache size for MTrie")
		}).
		Module("computation manager", func(node *cmd.FlowNodeBuilder) error {
			rt := runtime.NewInterpreterRuntime()
			vm, err := virtualmachine.New(rt, node.RootChainID.Chain())
			if err != nil {
				return err
			}

			computationManager = computation.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Me,
				node.State,
				vm,
				node.Storage.Blocks,
			)

			return nil
		}).
		Module("execution metrics", func(node *cmd.FlowNodeBuilder) error {
			collector = metrics.NewExecutionCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Component("execution state ledger", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			err := bootstrap.BootstrapExecutionDatabase(node.DB, node.RootCommit, node.RootBlock.Header)
			// Root block already loaded, can simply continued
			if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
				return nil, fmt.Errorf("could not bootstrap execution DB: %w", err)
			} else if err == nil {
				// Newly bootstrapped Execution DB. Make sure to load execution state
				if err := loadCheckpoint(node.BaseConfig.BootstrapDir, triedir); err != nil {
					return nil, fmt.Errorf("could load bootstrap checkpoint: %w", err)
				}
			}

			ledgerStorage, err = ledger.NewMTrieStorage(triedir, int(mTrieCacheSize), collector, node.MetricsRegisterer)
			return ledgerStorage, err
		}).
		Component("execution state ledger WAL compactor", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			checkpointer, err := ledgerStorage.Checkpointer()
			if err != nil {
				return nil, fmt.Errorf("cannot create checkpointer: %w", err)
			}
			compactor := wal.NewCompactor(checkpointer, 10*time.Second)

			return compactor, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkDataPacks := badger.NewChunkDataPacks(node.DB)
			executionResults := badger.NewExecutionResults(node.DB)
			stateCommitments := badger.NewCommits(node.Metrics.Cache, node.DB)

			executionState = state.NewExecutionState(
				ledgerStorage,
				stateCommitments,
				node.Storage.Blocks,
				node.Storage.Collections,
				chunkDataPacks,
				executionResults,
				node.DB,
				node.Tracer,
			)

			stateSync := sync.NewStateSynchronizer(executionState)

			providerEngine, err = provider.New(
				node.Logger,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				executionState,
				stateSync,
			)

			return providerEngine, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// Needed for gRPC server, make sure to assign to main scoped vars
			events = badger.NewEvents(node.DB)
			txResults = badger.NewTransactionResults(node.DB)
			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				node.Storage.Payloads,
				node.Storage.Collections,
				events,
				txResults,
				computationManager,
				providerEngine,
				executionState,
				6, // TODO - config param maybe?
				collector,
				node.Tracer,
				true,
				time.Second, //TODO - config param
				10,          // TODO - config param
			)
			return ingestionEng, err
		}).
		Component("grpc server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng, node.Storage.Blocks, events, txResults)
			return rpcEng, nil
		}).Run()

}

func loadCheckpoint(dir, trie string) error {
	src := filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, "checkpoint.00000000")
	dst := filepath.Join(trie, "checkpoint.00000000")

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
