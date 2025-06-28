#\!/bin/bash

# Export all processor types and update references
processors=(
    "aggregateProcessor:AggregateProcessor"
    "attestationProcessor:AttestationProcessor"
    "attesterSlashingProcessor:AttesterSlashingProcessor"
    "beaconBlockProcessor:BeaconBlockProcessor"
    "blsToExecutionProcessor:BlsToExecutionProcessor"
    "proposerSlashingProcessor:ProposerSlashingProcessor"
    "syncCommitteeProcessor:SyncCommitteeProcessor"
    "syncContributionProcessor:SyncContributionProcessor"
    "voluntaryExitProcessor:VoluntaryExitProcessor"
)

for pair in "${processors[@]}"; do
    old="${pair%:*}"
    new="${pair#*:}"
    
    # Export the type
    find pkg/consensus/mimicry/p2p/eth -name "*.go" -exec sed -i '' "s/type $old struct/type $new struct/g" {} \;
    
    # Update pointer references
    find pkg/consensus/mimicry/p2p/eth -name "*.go" -exec sed -i '' "s/\*$old/\*$new/g" {} \;
    
    # Update struct instantiations
    find pkg/consensus/mimicry/p2p/eth -name "*.go" -exec sed -i '' "s/\&$old/\&$new/g" {} \;
    
    # Update method receivers
    find pkg/consensus/mimicry/p2p/eth -name "*.go" -exec sed -i '' "s/(p \*$old)/(p \*$new)/g" {} \;
done

# Also update the comment for AggregateProcessor
sed -i '' 's/aggregateProcessor handles aggregate/AggregateProcessor handles aggregate/g' pkg/consensus/mimicry/p2p/eth/pubsub_aggregate.go
