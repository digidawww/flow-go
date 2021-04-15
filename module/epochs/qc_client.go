package epochs

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (

	// TransactionSubmissionTimeout is the time after which we return an error.
	TransactionSubmissionTimeout = 5 * time.Minute

	// TransactionStatusRetryTimeout is the time after which the status of a
	// transaction is checked again
	TransactionStatusRetryTimeout = 1 * time.Second
)

// QCContractClient is a client to the Quorum Certificate contract. Allows the client to
// functionality to submit a vote and check if collection node has voted already.
type QCContractClient struct {
	BaseContractClient

	nodeID flow.Identifier // flow identifier of the collection node

	env templates.Environment
}

// NewQCContractClient returns a new client to the Quorum Certificate contract
func NewQCContractClient(log zerolog.Logger,
	flowClient module.SDKClientWrapper,
	nodeID flow.Identifier,
	accountAddress string,
	accountKeyIndex uint,
	qcContractAddress string,
	signer sdkcrypto.Signer) (*QCContractClient, error) {

	base, err := NewBaseContractClient(log, flowClient, accountAddress, accountKeyIndex, signer, qcContractAddress)
	if err != nil {
		return nil, err
	}
	env := templates.Environment{QuorumCertificateAddress: qcContractAddress}

	return &QCContractClient{
		BaseContractClient: *base,
		nodeID:             nodeID,
		env:                env,
	}, nil
}

// SubmitVote submits the given vote to the cluster QC aggregator smart
// contract. This function returns only once the transaction has been
// processed by the network. An error is returned if the transaction has
// failed and should be re-submitted.
func (c *QCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {

	// time method was invoked
	started := time.Now()

	// add a timeout to the context
	ctx, cancel := context.WithTimeout(ctx, TransactionSubmissionTimeout)
	defer cancel()

	// get account for given address
	account, err := c.Client.GetAccount(ctx, c.Account.Address)
	if err != nil {
		return fmt.Errorf("could not get account: %w", err)
	}
	c.Account = account

	// get latest sealed block to execute transaction
	latestBlock, err := c.Client.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// attach submit vote transaction template and build transaction
	seqNumber := c.Account.Keys[int(c.AccountKeyIndex)].SequenceNumber
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSubmitVoteScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(c.Account.Address, int(c.AccountKeyIndex), seqNumber).
		SetPayer(c.Account.Address).
		AddAuthorizer(c.Account.Address)

	// add signature data to the transaction and submit to node
	err = tx.AddArgument(cadence.NewString(hex.EncodeToString(vote.SigData)))
	if err != nil {
		return fmt.Errorf("could not add raw vote data to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(c.Account.Address, int(c.AccountKeyIndex), c.Signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	txID, err := c.SendTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	err = c.WaitForSealed(ctx, txID, started)
	if err != nil {
		return fmt.Errorf("failed to wait for transaction seal: %w", err)
	}

	return nil
}

// Voted returns true if we have successfully submitted a vote to the
// cluster QC aggregator smart contract for the current epoch.
func (c *QCContractClient) Voted(ctx context.Context) (bool, error) {

	// execute script to read if voted
	arg := jsoncdc.MustEncode(cadence.String(c.nodeID.String()))
	template := templates.GenerateGetNodeHasVotedScript(c.env)
	hasVoted, err := c.Client.ExecuteScriptAtLatestBlock(ctx, template, []cadence.Value{cadence.String(arg)})
	if err != nil {
		return false, fmt.Errorf("could not execute voted script: %w", err)
	}

	// check if node has voted
	if !hasVoted.(cadence.Bool) {
		return false, nil
	}

	return true, nil
}
