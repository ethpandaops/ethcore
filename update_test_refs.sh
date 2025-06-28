#\!/bin/bash

# Update all processor references in test files
cd tests/p2p/eth

# Update type references
sed -i '' 's/aggregateProcessor/eth.AggregateProcessor/g' *.go
sed -i '' 's/attestationProcessor/eth.AttestationProcessor/g' *.go
sed -i '' 's/attesterSlashingProcessor/eth.AttesterSlashingProcessor/g' *.go
sed -i '' 's/beaconBlockProcessor/eth.BeaconBlockProcessor/g' *.go
sed -i '' 's/blsToExecutionProcessor/eth.BlsToExecutionProcessor/g' *.go
sed -i '' 's/proposerSlashingProcessor/eth.ProposerSlashingProcessor/g' *.go
sed -i '' 's/syncCommitteeProcessor/eth.SyncCommitteeProcessor/g' *.go
sed -i '' 's/syncContributionProcessor/eth.SyncContributionProcessor/g' *.go
sed -i '' 's/voluntaryExitProcessor/eth.VoluntaryExitProcessor/g' *.go

# Fix struct instantiations (remove eth prefix since we're already importing eth)
sed -i '' 's/&eth\.AggregateProcessor/\&eth.AggregateProcessor/g' *.go
sed -i '' 's/&eth\.AttestationProcessor/\&eth.AttestationProcessor/g' *.go
sed -i '' 's/&eth\.AttesterSlashingProcessor/\&eth.AttesterSlashingProcessor/g' *.go
sed -i '' 's/&eth\.BeaconBlockProcessor/\&eth.BeaconBlockProcessor/g' *.go
sed -i '' 's/&eth\.BlsToExecutionProcessor/\&eth.BlsToExecutionProcessor/g' *.go
sed -i '' 's/&eth\.ProposerSlashingProcessor/\&eth.ProposerSlashingProcessor/g' *.go
sed -i '' 's/&eth\.SyncCommitteeProcessor/\&eth.SyncCommitteeProcessor/g' *.go
sed -i '' 's/&eth\.SyncContributionProcessor/\&eth.SyncContributionProcessor/g' *.go
sed -i '' 's/&eth\.VoluntaryExitProcessor/\&eth.VoluntaryExitProcessor/g' *.go

# Update const references
sed -i '' 's/BeaconAggregateAndProofTopicName/eth.BeaconAggregateAndProofTopicName/g' *.go
sed -i '' 's/BeaconBlockTopicName/eth.BeaconBlockTopicName/g' *.go
sed -i '' 's/AttesterSlashingTopicName/eth.AttesterSlashingTopicName/g' *.go
sed -i '' 's/ProposerSlashingTopicName/eth.ProposerSlashingTopicName/g' *.go
sed -i '' 's/VoluntaryExitTopicName/eth.VoluntaryExitTopicName/g' *.go
sed -i '' 's/BlsToExecutionChangeTopicName/eth.BlsToExecutionChangeTopicName/g' *.go
sed -i '' 's/SyncContributionAndProofTopicName/eth.SyncContributionAndProofTopicName/g' *.go

# Update subnet constants
sed -i '' 's/AttestationSubnetCount/eth.AttestationSubnetCount/g' *.go
sed -i '' 's/SyncCommitteeSubnetCount/eth.SyncCommitteeSubnetCount/g' *.go
sed -i '' 's/AttestationSubnetTopicTemplate/eth.AttestationSubnetTopicTemplate/g' *.go
sed -i '' 's/SyncCommitteeSubnetTopicTemplate/eth.SyncCommitteeSubnetTopicTemplate/g' *.go
