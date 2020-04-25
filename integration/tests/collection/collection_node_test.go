package collection

import (
	"context"
	"testing"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/examples"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// the timeout for individual actions (eg. send a transaction)
	defaultTimeout = 10 * time.Second
	// the period we wait to give consensus/routing time to complete
	waitTime = 20 * time.Second
)

// default set of non-collection nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel))
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, exeNode, verNode}
}

// Test sending various invalid transactions to a single-cluster configuration.
// The transactions should be rejected by the collection node and not included
// in any collection.
func TestTransactionIngress_InvalidTransaction(t *testing.T) {

	colNodeConfigs := testnet.NewNodeConfigSet(3, flow.RoleCollection)
	colNodeConfig1 := colNodeConfigs[0]
	nodes := append(colNodeConfigs, defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig("col_txingress_invalidtx", nodes)

	net := testnet.PrepareFlowNetwork(t, conf)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Remove()

	// we will test against COL1
	colNode1 := net.ContainerByID(colNodeConfig1.Identifier)

	client, err := client.New(colNode1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	addr, acct, signer := getAccount()

	t.Run("missing reference block hash", func(t *testing.T) {
		malformed := sdk.NewTransaction().
			SetScript(unittest.NoopTxScript()).
			SetProposalKey(addr, acct.ID, acct.SequenceNumber).
			SetPayer(addr).
			AddAuthorizer(addr)

		malformed.SetReferenceBlockID(sdk.ZeroID)

		err = malformed.SignEnvelope(sdk.RootAddress, acct.ID, signer)
		require.Nil(t, err)

		expected := ingest.ErrIncompleteTransaction{
			Missing: []string{flow.TransactionFieldRefBlockID.String()},
		}

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("missing script", func(t *testing.T) {
		malformed := sdk.NewTransaction().
			SetReferenceBlockID(sdk.Identifier{1}).
			SetProposalKey(sdk.RootAddress, acct.ID, acct.SequenceNumber).
			SetPayer(sdk.RootAddress).
			AddAuthorizer(sdk.RootAddress)

		malformed.SetScript(nil)

		expected := ingest.ErrIncompleteTransaction{
			Missing: []string{flow.TransactionFieldScript.String()},
		}

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("unparseable script", func(t *testing.T) {
		// TODO script parsing not implemented
		t.Skip()
	})
	t.Run("invalid signature", func(t *testing.T) {
		// TODO signature validation not implemented
		t.Skip()
	})
	t.Run("invalid nonce", func(t *testing.T) {
		// TODO nonce validation not implemented
		t.Skip()
	})
	t.Run("insufficient payer balance", func(t *testing.T) {
		// TODO balance checking not implemented
		t.Skip()
	})
	t.Run("expired transaction", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("non-existent reference block ID", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
}

// Test sending a single valid transaction to a single cluster.
// The transaction should be included in a collection.
func TestTxIngress_SingleCluster(t *testing.T) {

	colNodeConfigs := testnet.NewNodeConfigSet(3, flow.RoleCollection)
	colNodeConfig1 := colNodeConfigs[0]
	nodes := append(colNodeConfigs, defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig("col_txingress_singlecluster", nodes)

	net := testnet.PrepareFlowNetwork(t, conf)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will test against COL1
	colNode1 := net.ContainerByID(colNodeConfig1.Identifier)

	client, err := client.New(colNode1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	addr, acct, signer := getAccount()

	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(sdk.BytesToID([]byte{1})).
		SetProposalKey(addr, acct.ID, acct.SequenceNumber).
		SetPayer(addr).
		AddAuthorizer(addr)

	err = tx.SignEnvelope(addr, acct.ID, signer)
	require.Nil(t, err)

	t.Log("sending transaction: ", tx.ID())

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err = client.SendTransaction(ctx, *tx)
	assert.Nil(t, err)

	// wait for consensus to complete
	//TODO we should listen for collection guarantees instead, but this is blocked
	// ref: https://github.com/dapperlabs/flow-go/issues/3021
	time.Sleep(waitTime)

	net.Stop()

	identities := net.Identities()
	chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))

	// get database for COL1
	db, err := colNode1.DB()
	require.Nil(t, err)

	state, err := clusterstate.NewState(db, chainID)
	assert.Nil(t, err)

	// TODO utility for this?
	txID, err := flow.HexStringToIdentifier(tx.ID().String())
	assert.Nil(t, err)

	// the transaction should be included in exactly one collection
	checker := unittest.NewClusterStateChecker(state)
	checker.
		ExpectContainsTx(txID).
		ExpectTxCount(1).
		Assert(t)
}

// Test sending a single valid transaction to the responsible cluster in a
// multi-cluster configuration
//
// The transaction should not be routed and should be included in exactly one
// collection in only the responsible cluster.
func TestTxIngressMultiCluster_CorrectCluster(t *testing.T) {

	const nClusters uint = 3

	colNodes := testnet.NewNodeConfigSet(6, flow.RoleCollection)

	nodes := append(colNodes, defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig("col_txingres_multicluster_correct_cluster", nodes, testnet.WithClusters(nClusters))

	net := testnet.PrepareFlowNetwork(t, conf)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// sleep for a few seconds to let the nodes discover each other and build the libp2p pubsub mesh
	time.Sleep(5 * time.Second)

	clusters := protocol.Clusters(nClusters, net.Identities())

	// pick a cluster to target
	targetCluster := clusters.ByIndex(0)
	targetIdentity, ok := targetCluster.ByIndex(0)
	require.True(t, ok)

	// pick a member of the cluster
	targetNode := net.ContainerByID(targetIdentity.NodeID)

	// get a client pointing to the cluster member
	client, err := client.New(targetNode.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	addr, acct, signer := getAccount()

	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(sdk.Identifier{1}).
		SetProposalKey(addr, acct.ID, acct.SequenceNumber).
		SetPayer(addr).
		AddAuthorizer(addr)

	// hash-grind the script until the transaction will be routed to target cluster
	for {
		tx.SetScript(append(tx.Script, '/', '/'))
		err = tx.SignEnvelope(sdk.RootAddress, acct.ID, signer)
		require.Nil(t, err)
		routed := clusters.ByTxID(convert.IDFromSDK(tx.ID()))
		if routed.Fingerprint() == targetCluster.Fingerprint() {
			break
		}
	}

	// submit the transaction
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err = client.SendTransaction(ctx, *tx)
	assert.Nil(t, err)

	// wait for consensus to complete
	time.Sleep(waitTime)
	net.Stop()

	// ensure the transaction IS included in target cluster collection
	for _, id := range targetCluster.NodeIDs() {
		node := net.ContainerByID(id)
		db, err := node.DB()
		require.Nil(t, err)

		chainID := protocol.ChainIDForCluster(targetCluster)
		state, err := clusterstate.NewState(db, chainID)
		require.Nil(t, err)

		// the transaction should be included exactly once
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(convert.IDFromSDK(tx.ID())).
			ExpectTxCount(1).
			Assert(t)
	}

	// ensure the transaction IS NOT included in other cluster collections
	for _, cluster := range clusters.All() {
		// skip the target cluster
		if cluster.Fingerprint() == targetCluster.Fingerprint() {
			continue
		}

		chainID := protocol.ChainIDForCluster(cluster)

		for _, id := range cluster.NodeIDs() {
			node := net.ContainerByID(id)
			db, err := node.DB()
			require.Nil(t, err)

			state, err := clusterstate.NewState(db, chainID)
			require.Nil(t, err)

			// the transaction should not be included
			checker := unittest.NewClusterStateChecker(state)
			checker.
				ExpectOmitsTx(convert.IDFromSDK(tx.ID())).
				ExpectTxCount(0).
				Assert(t)
		}
	}
}

// Test sending a single valid transaction to a non-responsible cluster in a
// multi-cluster configuration
//
// The transaction should be routed to the responsible cluster and should be
// included in a collection in only the responsible cluster's state.
func TestTxIngressMultiCluster_OtherCluster(t *testing.T) {

	const nClusters uint = 3

	colNodes := testnet.NewNodeConfigSet(6, flow.RoleCollection)

	nodes := append(colNodes, defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig("col_txingress_multicluster_othercluster", nodes, testnet.WithClusters(nClusters))

	net := testnet.PrepareFlowNetwork(t, conf)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	clusters := protocol.Clusters(nClusters, net.Identities())

	// pick a cluster to target
	// this cluster is responsible for the transaction
	targetCluster := clusters.ByIndex(0)

	// pick nodes from the other clusters
	// these nodes will receive the transaction, but are not responsible for it
	otherCluster1 := clusters.ByIndex(1)
	otherIdentity1, ok := otherCluster1.ByIndex(0)
	require.True(t, ok)
	otherNode1 := net.ContainerByID(otherIdentity1.NodeID)

	otherCluster2 := clusters.ByIndex(2)
	otherIdentity2, ok := otherCluster2.ByIndex(0)
	require.True(t, ok)
	otherNode2 := net.ContainerByID(otherIdentity2.NodeID)

	addr, acct, signer := getAccount()

	// create clients pointing to each other node
	client1, err := client.New(otherNode1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)
	client2, err := client.New(otherNode2.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	// create a transaction for the target cluster
	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(sdk.Identifier{1}).
		SetProposalKey(addr, acct.ID, acct.SequenceNumber).
		SetPayer(addr).
		AddAuthorizer(addr)

	// hash-grind the script until the transaction will be routed to target cluster
	for {
		tx.SetScript(append(tx.Script, '/', '/'))
		err = tx.SignEnvelope(addr, acct.ID, signer)
		require.Nil(t, err)
		routed := clusters.ByTxID(convert.IDFromSDK(tx.ID()))
		if routed.Fingerprint() == targetCluster.Fingerprint() {
			break
		}
	}

	go func() {
		for i := 0; i < 3; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second * 2):
			}
			// submit the transaction to other cluster 1
			ctx, _ := context.WithTimeout(ctx, defaultTimeout)
			err = client1.SendTransaction(ctx, *tx)
			assert.Nil(t, err)

			// submit the transaction to other cluster 2
			ctx, _ = context.WithTimeout(ctx, defaultTimeout)
			err = client2.SendTransaction(ctx, *tx)
			assert.Nil(t, err)
		}
	}()

	// wait for consensus to complete
	time.Sleep(waitTime)
	net.Stop()

	// ensure the transaction IS included in target cluster collection
	for _, id := range targetCluster.NodeIDs() {
		node := net.ContainerByID(id)
		db, err := node.DB()
		require.Nil(t, err)

		chainID := protocol.ChainIDForCluster(targetCluster)
		state, err := clusterstate.NewState(db, chainID)
		require.Nil(t, err)

		// the transaction should be included exactly once
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(convert.IDFromSDK(tx.ID())).
			ExpectTxCount(1).
			Assert(t)
	}

	// ensure the transaction IS NOT included in other cluster collections
	for _, cluster := range clusters.All() {
		// skip the target cluster
		if cluster.Fingerprint() == targetCluster.Fingerprint() {
			continue
		}

		chainID := protocol.ChainIDForCluster(cluster)

		for _, id := range cluster.NodeIDs() {
			node := net.ContainerByID(id)
			db, err := node.DB()
			require.Nil(t, err)

			state, err := clusterstate.NewState(db, chainID)
			require.Nil(t, err)

			// the transaction should not be included
			checker := unittest.NewClusterStateChecker(state)
			checker.
				ExpectOmitsTx(convert.IDFromSDK(tx.ID())).
				ExpectTxCount(0).
				Assert(t)
		}
	}
}

func getAccount() (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer) {

	addr := convert.ToSDKAddress(unittest.AddressFixture())

	key := examples.RandomPrivateKey()
	signer := sdkcrypto.NewInMemorySigner(key, sdkcrypto.SHA3_256)

	acct := sdk.NewAccountKey().
		FromPrivateKey(key).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	return addr, acct, signer
}
