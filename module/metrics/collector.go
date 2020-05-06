package metrics

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/module"

	"github.com/dapperlabs/flow-go/module/trace"
)

// Collectors implement the module.Metrics interface.
// They provides methods for collecting metrics data for monitoring purposes.
// A collector is composed of BaseMetrics in conjunction with different implementations for
// collecting HotStuff metrics:
//   * HotStuffNoopMetrics are used for nodes which don't actively participate in hotstuff
//   * HotStuffMetrics are used for Conesnsus
//
// This implementation takes a SHORT CUT:
// [CONTEXT] We have multiple instances of HotStuff running within Flow: Consensus Nodes form
// the main consensus committee. In addition each Collector Node cluster runs their
// own HotStuff instance. Depending on the node role, the name space is different. Furthermore,
// even within the `collection` name space, we need to separate metrics between the different
// clusters. We do this by adding the label `committeeID` to the HotStuff metrics and
// allowing for configurable name space.
// In contrast, for all other metrics, the name space and labels are fixed.
// [CURRENT SOLUTION]
// We separate metrics into a set of BaseMetrics and HotStuff-specific metrics.
// The implementation of the BaseMetrics is shared for all nodes.
// Only collector nodes will generate metrics, which require the cluster ID as Label.
// For all other node types, the clusterID is not used and not set.

// ToDo: Clean up Tech Dept: Split up the module.Metrics interface and make one dedicated
//       Interface for each node role. Then, BaseMetrics can be split up to implement only
//       the metrics that are actually generated by the respective node role.
type BaseMetrics struct {
	tracer trace.Tracer
}

// NewCollector instantiates a new module.Metrics implementation.
// ONLY SUITABLE FOR nodes WITHOUT HOTSTUFF
func NewCollector(log zerolog.Logger) (module.Metrics, error) {
	tracer, err := trace.NewTracer(log)
	if err != nil {
		return nil, err
	}

	// anonymous struct implementing module.Metrics interface.
	metricsCollector := struct {
		BaseMetrics
		HotStuffNoopMetrics
	}{
		BaseMetrics: BaseMetrics{tracer},
	}
	return &metricsCollector, nil
}

// NewConsensusCollector instantiates a new metrics Collector ONLY suitable for CONSENSUS NODES!
func NewConsensusCollector(log zerolog.Logger) (module.Metrics, error) {
	tracer, err := trace.NewTracer(log)
	if err != nil {
		return nil, err
	}

	// constant labels for metrics
	metricLabels := map[string]string{
		"chain_id":    mainConsensusCommittee,
		"node_role":   flow.RoleConsensus.String(),
		"beta_metric": "true",
	}

	// anonymous struct implementing module.Metrics interface.
	metricsCollector := struct {
		BaseMetrics
		HotStuffMetrics
	}{
		BaseMetrics:     BaseMetrics{tracer},
		HotStuffMetrics: NewHotStuffMetrics(metricLabels),
	}
	return &metricsCollector, nil
}

// NewClusterCollector instantiates a new metrics Collector ONLY suitable for COLLECTOR NODES!
func NewClusterCollector(log zerolog.Logger, clusterID string) (module.Metrics, error) {
	tracer, err := trace.NewTracer(log)
	if err != nil {
		return nil, err
	}

	// constant labels for metrics
	metricLabels := map[string]string{
		"chain_id":    clusterID,
		"node_role":   flow.RoleCollection.String(),
		"beta_metric": "true",
	}

	// anonymous struct implementing module.Metrics interface.
	metricsCollector := struct {
		BaseMetrics
		HotStuffMetrics
	}{
		BaseMetrics:     BaseMetrics{tracer},
		HotStuffMetrics: NewHotStuffMetrics(metricLabels),
	}
	return &metricsCollector, nil
}
