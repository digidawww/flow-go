package badger

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(5, unittest.WithAllRoles())

func TestEpochSetupValidity(t *testing.T) {
	t.Run("invalid first/final view", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// set an invalid final view for the first epoch
		setup.FinalView = setup.FirstView

		err := isValidEpochSetup(setup)
		require.Error(t, err)
	})

	t.Run("invalid cluster assignments", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// create an invalid cluster assignment (node appears in multiple clusters)
		collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
		setup.Assignments = append(setup.Assignments, []flow.Identifier{collector.NodeID})

		err := isValidEpochSetup(setup)
		require.Error(t, err)
	})

	t.Run("short seed", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.RandomSource = unittest.SeedFixture(crypto.SeedMinLenDKG - 1)

		err := isValidEpochSetup(setup)
		require.Error(t, err)
	})
}

func TestBootstrapInvalidEpochCommit(t *testing.T) {
	t.Run("inconsistent counter", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// use a different counter for the commit
		commit.Counter = setup.Counter + 1

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent cluster QCs", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra QC to commit
		commit.ClusterQCs = append(commit.ClusterQCs, unittest.QuorumCertificateFixture())

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("missing dkg group key", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		commit.DKGGroupKey = nil

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent DKG participants", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an invalid DKG participant
		collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
		commit.DKGParticipants[collector.NodeID] = flow.DKGParticipant{
			KeyShare: unittest.KeyFixture(crypto.BLSBLS12381).PublicKey(),
			Index:    1,
		}

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})
}
