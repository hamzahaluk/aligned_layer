package pkg

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/Layr-Labs/eigensdk-go/crypto/bls"
	eigentypes "github.com/Layr-Labs/eigensdk-go/types"
	retry "github.com/yetanotherco/aligned_layer/core"
	"github.com/yetanotherco/aligned_layer/core/types"
	"net/http"
	"net/rpc"
	"strings"
	"time"
)

func (agg *Aggregator) ServeOperators() error {
	// Registers a new RPC server
	err := rpc.Register(agg)
	if err != nil {
		return err
	}

	// Registers an HTTP handler for RPC messages
	rpc.HandleHTTP()

	// Start listening for requests on aggregator address
	// ServeOperators accepts incoming HTTP connections on the listener, creating
	// a new service goroutine for each. The service goroutines read requests
	// and then call handler to reply to them
	agg.logger.Info("Starting RPC server on address", "address",
		agg.AggregatorConfig.Aggregator.ServerIpPortAddress)

	err = http.ListenAndServe(agg.AggregatorConfig.Aggregator.ServerIpPortAddress, nil)

	return err
}

// ~~ AGGREGATOR METHODS ~~
// This is the list of methods that the Aggregator exposes to the Operator
// The Operator can call these methods to interact with the Aggregator
// This methods are automatically registered by the RPC server

// Takes a response from an operator and process it. After processing the response, the associated task may reach quorum, triggering a BLS service response.
// If the task related to the response is not known to the aggregator (not stored in internal map), it will try to fetch it from the logs.
// Returns:
//   - 0: Success
//   - 1: Error
func (agg *Aggregator) ProcessOperatorSignedTaskResponseV2(signedTaskResponse *types.SignedTaskResponse, reply *uint8) error {
	agg.AggregatorConfig.BaseConfig.Logger.Info("New task response",
		"BatchMerkleRoot", "0x"+hex.EncodeToString(signedTaskResponse.BatchMerkleRoot[:]),
		"SenderAddress", "0x"+hex.EncodeToString(signedTaskResponse.SenderAddress[:]),
		"BatchIdentifierHash", "0x"+hex.EncodeToString(signedTaskResponse.BatchIdentifierHash[:]),
		"operatorId", hex.EncodeToString(signedTaskResponse.OperatorId[:]))

	agg.taskMutex.Lock()
	agg.AggregatorConfig.BaseConfig.Logger.Info("- Locked Resources: Process operator response")
	// Unlock mutex after returning
	defer func() {
		agg.taskMutex.Unlock()
		agg.logger.Info("- Unlocked Resources: Process operator response")
	}()

	taskIndex, err := agg.GetTaskIndex(signedTaskResponse.BatchIdentifierHash)

	if err != nil {
		agg.logger.Warn("Task not found in the internal map, might have been missed. Trying to fetch task data from Ethereum")
		batch, err := agg.avsReader.GetPendingBatchFromMerkleRoot(signedTaskResponse.BatchMerkleRoot, agg.AggregatorConfig.Aggregator.PendingBatchFetchBlockRange)
		if err != nil || batch == nil {
			agg.logger.Warnf("Pending task with merkle root 0x%x not found in logs", signedTaskResponse.BatchMerkleRoot)
			*reply = 1
			return nil
		}
		agg.logger.Info("Task was found in Ethereum, adding it to the internal map")
		agg.AddNewTask(batch.BatchMerkleRoot, batch.SenderAddress, batch.TaskCreatedBlock)
		taskIndex, err = agg.GetTaskIndex(signedTaskResponse.BatchIdentifierHash)
		if err != nil {
			// This shouldn't happen, since we just added the task
			agg.logger.Error("Unexpected error trying to get taskIndex from internal map")
			*reply = 1
			return nil
		}
	}

	agg.telemetry.LogOperatorResponse(signedTaskResponse.BatchMerkleRoot, signedTaskResponse.OperatorId)

	// Don't wait infinitely if it can't answer
	// Create a context with a timeout of 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	// Create a channel to signal when the task is done
	done := make(chan struct{})

	agg.logger.Info("Starting bls signature process")
	go func() {
		err := agg.ProcessNewSignature(
			context.Background(), taskIndex, signedTaskResponse.BatchIdentifierHash,
			&signedTaskResponse.BlsSignature, signedTaskResponse.OperatorId,
		)

		if err != nil {
			agg.logger.Warnf("BLS aggregation service error: %s", err)
			// todo shouldn't we here close the channel with a reply = 1?
		} else {
			agg.logger.Info("BLS process succeeded")
		}

		close(done)
	}()

	*reply = 1
	// Wait for either the context to be done or the task to complete
	select {
	case <-ctx.Done():
		// The context's deadline was exceeded or it was canceled
		agg.logger.Info("Bls process timed out, operator signature will be lost. Batch may not reach quorum")
	case <-done:
		// The task completed successfully
		agg.logger.Info("Bls context finished correctly")
		*reply = 0
	}

	return nil
}

// Dummy method to check if the server is running
// TODO: Remove this method in prod
func (agg *Aggregator) ServerRunning(_ *struct{}, reply *int64) error {
	*reply = 1
	return nil
}

// |---RETRYABLE---|

/*
  - Errors:
    Permanent:
  - SignatureVerificationError: Verification of the sigature within the BLS Aggregation Service failed. (https://github.com/Layr-Labs/eigensdk-go/blob/dev/services/bls_aggregation/blsagg.go#L42).
    Transient:
  - All others.
  - Retry times (3 retries): 12 sec (1 Blocks), 24 sec (2 Blocks), 48 sec (4 Blocks)
  - NOTE: TaskNotFound errors from the BLS Aggregation service are Transient errors as block reorg's may lead to these errors being thrown.
*/
func (agg *Aggregator) ProcessNewSignature(ctx context.Context, taskIndex uint32, taskResponse interface{}, blsSignature *bls.Signature, operatorId eigentypes.Bytes32) error {
	processNewSignature_func := func() error {
		err := agg.blsAggregationService.ProcessNewSignature(
			ctx, taskIndex, taskResponse,
			blsSignature, operatorId,
		)
		if err != nil {
			if strings.Contains(err.Error(), "Failed to verify signature") {
				err = retry.PermanentError{Inner: err}
			}
		}
		return err
	}

	return retry.Retry(processNewSignature_func, retry.MinDelayChain, retry.RetryFactor, retry.NumRetries, retry.MaxIntervalChain, retry.MaxElapsedTime)
}

func (agg *Aggregator) GetTaskIndex(batchIdentifierHash [32]byte) (uint32, error) {
	getTaskIndex_func := func() (uint32, error) {
		taskIndex, ok := agg.batchesIdxByIdentifierHash[batchIdentifierHash]
		if !ok {
			return taskIndex, fmt.Errorf("Task not found in the internal map")
		} else {
			return taskIndex, nil
		}
	}

	return retry.RetryWithData(getTaskIndex_func, retry.MinDelay, retry.RetryFactor, retry.NumRetries, retry.MaxInterval, retry.MaxElapsedTime)
}
