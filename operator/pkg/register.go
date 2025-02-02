package operator

import (
	"context"

	"github.com/Layr-Labs/eigensdk-go/types"
	"github.com/yetanotherco/aligned_layer/core/chainio"
	"github.com/yetanotherco/aligned_layer/core/config"
)

// RegisterOperator operator registers the operator with the given public key for the given quorum IDs.
// RegisterOperator registers a new operator with the given public key and socket with the provided quorum ids.
// If the operator is already registered with a given quorum id, the transaction will fail (noop) and an error
// will be returned.
func RegisterOperator(
	ctx context.Context,
	configuration *config.OperatorConfig,
	ecdsaConfig *config.EcdsaConfig,
	operatorToAvsRegistrationSigSalt [32]byte,
) error {
	writer, err := chainio.NewAvsWriterFromConfig(configuration.BaseConfig, ecdsaConfig, nil)
	if err != nil {
		configuration.BaseConfig.Logger.Error("Failed to create AVS writer", "err", err)
		return err
	}

	socket := "Not Needed"

	quorumNumbers := types.QuorumNums{0}

	_, err = writer.RegisterOperator(ctx, ecdsaConfig.PrivateKey,
		configuration.BlsConfig.KeyPair,
		quorumNumbers, socket, true)

	if err != nil {
		configuration.BaseConfig.Logger.Error("Failed to register operator", "err", err)
		return err
	}

	return nil
}
