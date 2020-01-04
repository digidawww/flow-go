package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestBlockExecutorExecuteBlock(t *testing.T) {
	vm := &vmmock.VirtualMachine{}
	bc := &vmmock.BlockContext{}

	exe := executor.NewBlockExecutor(vm)

	tx1 := &flow.Transaction{
		TransactionBody: flow.TransactionBody{
			Script: []byte("transaction { execute {} }"),
		},
	}

	tx2 := &flow.Transaction{
		TransactionBody: flow.TransactionBody{
			Script: []byte("transaction { execute {} }"),
		},
	}

	col := &flow.Collection{Transactions: []flow.Fingerprint{
		tx1.Fingerprint(),
		tx2.Fingerprint(),
	}}

	block := &flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		GuaranteedCollections: []*flow.GuaranteedCollection{
			{
				CollectionHash: crypto.Hash(col.Fingerprint()),
				Signatures:     nil,
			},
		},
	}

	collections := []*flow.Collection{col}
	transactions := []*flow.Transaction{tx1, tx2}

	vm.On("NewBlockContext", block).
		Return(bc).
		Once()

	bc.On(
		"ExecuteTransaction",
		mock.AnythingOfType("*ledger.View"),
		mock.AnythingOfType("*flow.Transaction"),
	).
		Return(&virtualmachine.TransactionResult{}, nil).
		Twice()

	chunks, err := exe.ExecuteBlock(block, collections, transactions)
	assert.NoError(t, err)
	assert.Len(t, chunks, 1)

	chunk := chunks[0]
	assert.EqualValues(t, chunk.TxCounts, 2)

	vm.AssertExpectations(t)
	bc.AssertExpectations(t)
}
