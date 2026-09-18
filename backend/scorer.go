package aegiscortex

import (
	"context"
	"os"
	"strings"

	"github.com/orcawhisperer/typesafe-sdk-go"
)

// SpeculativeScorer is a pluggable fan-out. AegisCortex owns routing,
// field lock, and τ. The scorer only returns calibrated probabilities.
type SpeculativeScorer interface {
	Name() string
	Score(ctx context.Context, pipeline PipelineID, state any, questions typesafe.Questions, specs []BoundQuestionSpec) (*typesafe.SystemOneResponse, error)
}

// EngineKeyFromEnv prefers AEGIS_ENGINE_KEY, then TYPESAFE_API_KEY.
func EngineKeyFromEnv() string {
	if k := strings.TrimSpace(os.Getenv("AEGIS_ENGINE_KEY")); k != "" {
		return k
	}
	if k := strings.TrimSpace(os.Getenv(typesafe.APIKeyEnv)); k != "" {
		return k
	}
	return strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY"))
}

type simulatorScorer struct{}

func (simulatorScorer) Name() string { return "aegis_calibrated_simulator" }

func (simulatorScorer) Score(_ context.Context, pipeline PipelineID, state any, _ typesafe.Questions, specs []BoundQuestionSpec) (*typesafe.SystemOneResponse, error) {
	return simulateCalibratedJevResponse(pipeline, state, specs), nil
}

type typesafeScorer struct {
	engine *CortexEngine
}

func (typesafeScorer) Name() string { return "typesafe_jev" }

func (s typesafeScorer) Score(ctx context.Context, _ PipelineID, state any, questions typesafe.Questions, _ []BoundQuestionSpec) (*typesafe.SystemOneResponse, error) {
	client, err := s.engine.acquireLiveClient()
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, nil
	}
	return client.SystemOne(ctx, typesafe.SystemOneRequest{
		State:     state,
		Questions: questions,
		Model:     typesafe.ModelJev1_13_0,
	})
}
