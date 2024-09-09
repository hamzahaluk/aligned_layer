# Integrating Aligned into your Application

Aligned can be integrated into your applications in a few simple steps to provide a way to verify ZK proofs generated inside your system.

You can find an example of the full flow of using Aligned in your app in the [ZKQuiz example](../../examples/zkquiz).

This example shows a sample app that generates an SP1 proof that a user knows the answers to a quiz and then submits the proof to Aligned for verification. Finally, it includes a smart contract that verifies that a proof was verified in Aligned and mints an NFT.

## 1. Generate your ZK Proof

To submit proofs to Aligned and get them verified, first you need to generate those proofs. Every proving system has its own way of generating proofs.

You can find examples on how to generate proofs in the [generating proofs guide](4_generating_proofs.md).

Also, you can find an example of the ZKQuiz proof [program](../../examples/zkquiz/quiz/program/src/main.rs) as well as the [script](../../examples/zkquiz/quiz/script/src/main.rs) that generates it in the [ZKQuiz example directory](../../examples/zkquiz).

## 2. Write your smart contract

To check if a proof was verified in Aligned, you need to make a call to the `AlignedServiceManager` contract inside your smart contract.

Also, you will need a way to check that the proven program is the correct one.

The Aligned CLI provides a way for you to get the verification key commitment without actually generating and submitting a proof.

You can do this by running the following command:

```bash
aligned get-commitment --input <path_to_input_file>
```

The following is an example of how to call the `verifyBatchInclusionMethod` from the `AlignedServiceManager` contract in your smart contract.

```solidity
contract YourContract {
    // Your contract variables ...
    address public alignedServiceManager;
    bytes32 public elfCommitment = <elf_commitment>;

    constructor(address _alignedServiceManager) {
        //... Your contract constructor ...
        alignedServiceManager = _alignedServiceManager;
    }

    // Your contract code ...

    function yourContractMethod(
        //... Your function variables, ...
        bytes32 proofCommitment,
        bytes32 pubInputCommitment,
        bytes32 provingSystemAuxDataCommitment,
        bytes20 proofGeneratorAddr,
        bytes32 batchMerkleRoot,
        bytes memory merkleProof,
        uint256 verificationDataBatchIndex
    ) {
        // ... Your function code

        require(elfCommitment == provingSystemAuxDataCommitment, "ELF does not match");

        (bool callWasSuccessful, bytes memory proofIsIncluded) = alignedServiceManager.staticcall(
            abi.encodeWithSignature(
                "verifyBatchInclusion(bytes32,bytes32,bytes32,bytes20,bytes32,bytes,uint256)",
                proofCommitment,
                pubInputCommitment,
                provingSystemAuxDataCommitment,
                proofGeneratorAddr,
                batchMerkleRoot,
                merkleProof,
                verificationDataBatchIndex
            )
        );

        require(callWasSuccessful, "static_call failed");

        bool proofIsIncludedBool = abi.decode(proofIsIncluded, (bool));
        require(proofIsIncludedBool, "proof not included in batch");

        // Your function code ...
    }
}
```

You can find an example of the smart contract that checks if the proof was verified in Aligned in the [Quiz Verifier Contract](../../examples/zkquiz/contracts/src/VerifierContract.sol).

Note that the contract checks if the verification key commitment is the same as the program ELF commitment.

```solidity
require(elfCommitment == provingSystemAuxDataCommitment, "ELF does not match");
```

This contract also includes a static call to the `AlignedServiceManager` contract to check if the proof was verified in Aligned.

```solidity
(bool callWasSuccessfull, bytes memory proofIsIncluded) = alignedServiceManager.staticcall(
    abi.encodeWithSignature(
        "verifyBatchInclusion(bytes32,bytes32,bytes32,bytes20,bytes32,bytes,uint256)",
        proofCommitment,
        pubInputCommitment,
        provingSystemAuxDataCommitment,
        proofGeneratorAddr,
        batchMerkleRoot,
        merkleProof,
        verificationDataBatchIndex
    )
);

require(callWasSuccessfull, "static_call failed");

bool proofIsIncludedBool = abi.decode(proofIsIncluded, (bool));
require(proofIsIncludedBool, "proof not included in batch");
```

## 3. Submit and verify the proof to Aligned

The proof submission and verification can be done either with the SDK or by using the Aligned CLI.

#### Using the SDK

To submit a proof using the SDK, you can use the `submit_and_wait_verification` function.
This function submits the proof to aligned and waits for it to be verified in Aligned.
Alternatively you can call `submit` if you dont want to wait for proof verification.
The following code is an example of how to submit a proof using the SDK:

```rust
use aligned_sdk::sdk::{submit_and_wait_verification, get_next_nonce};
use aligned_sdk::types::{ProvingSystemId, VerificationData};
use ethers::prelude::*;

const RPC_URL: &str = "https://ethereum-holesky-rpc.publicnode.com";
const BATCHER_URL: &str = "wss://batcher.alignedlayer.com";
const BATCHER_ADDRESS: &str = "0x815aeCA64a974297942D2Bbf034ABEe22a38A003";
const ELF: &[u8] = include_bytes!("../../program/elf/riscv32im-succinct-zkvm-elf");

async fn submit_proof_to_aligned(
    proof: Vec<u8>,
    wallet: Wallet<SigningKey>
) -> Result<AlignedVerificationData, anyhow::Error> {
    let verification_data = VerificationData {
        proving_system: ProvingSystemId::SP1,
        proof,
        proof_generator_addr: wallet.address(),
        vm_program_code: Some(ELF.to_vec()),
        verification_key: None,
        pub_input: None,
    };

    let nonce = get_next_nonce(RPC_URL, wallet.address(), BATCHER_CONTRACT_ADDRESS)
        .await
        .map_err(|e| anyhow::anyhow!("Failed to get next nonce: {:?}", e))?;

    match submit_and_wait_verification(
        BATCHER_URL,
        &rpc_url,
        Chain::Holesky,
        &verification_data,
        wallet.clone(),
        nonce,
        BATCHER_PAYMENTS_ADDRESS
    )

    submit_and_wait_verification(
        BATCHER_URL,
        RPC_URL,
        Chain::Holesky,
        &verification_data,
        wallet,
        nonce,
        BATCHER_CONTRACT_ADDRESS
    ).await.map_err(|e| anyhow::anyhow!("Failed to submit proof: {:?}", e))
}

#[tokio::main]
async fn main() {
    let wallet = // Initialize wallet

    let wallet = wallet.with_chain_id(17000u64)

    let proof = // Generate or obtain proof

    match submit_proof_to_aligned(proof, wallet).await {
        Ok(aligned_verification_data) => println!("Proof submitted successfully"),
        Err(err) => println!("Error: {:?}", err),
    }
}
```

You can find an example of the proof submission and verification in the [ZKQuiz Program](../../examples/zkquiz/quiz/script/src/main.rs).

This example generates a proof, instantiates a wallet to submit the proof, and then submits the proof to Aligned for verification. It then waits for the proof to be verified in Aligned.

#### Using the CLI

You can find examples of how to submit a proof using the CLI in the [submitting proofs guide](0_submitting_proofs.md).