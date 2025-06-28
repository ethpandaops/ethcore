#\!/bin/bash

# Export all fields in processor types
cd pkg/consensus/mimicry/p2p/eth

# Export common fields
find . -name "*.go" -exec sed -i '' 's/forkDigest \[4\]byte/ForkDigest [4]byte/g' {} \;
find . -name "*.go" -exec sed -i '' 's/encoder encoder\.SszNetworkEncoder/Encoder encoder.SszNetworkEncoder/g' {} \;
find . -name "*.go" -exec sed -i '' 's/handler func/Handler func/g' {} \;
find . -name "*.go" -exec sed -i '' 's/validator func/Validator func/g' {} \;
find . -name "*.go" -exec sed -i '' 's/gossipsub \*pubsub\.Gossipsub/Gossipsub *pubsub.Gossipsub/g' {} \;
find . -name "*.go" -exec sed -i '' 's/log logrus\.FieldLogger/Log logrus.FieldLogger/g' {} \;
find . -name "*.go" -exec sed -i '' 's/scoreParams \*pubsub\.TopicScoreParams/ScoreParams *pubsub.TopicScoreParams/g' {} \;
find . -name "*.go" -exec sed -i '' 's/subnets \[\]uint64/Subnets []uint64/g' {} \;

# Update field references in methods
find . -name "*.go" -exec sed -i '' 's/p\.forkDigest/p.ForkDigest/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.encoder/p.Encoder/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.handler/p.Handler/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.validator/p.Validator/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.gossipsub/p.Gossipsub/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.log/p.Log/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.scoreParams/p.ScoreParams/g' {} \;
find . -name "*.go" -exec sed -i '' 's/p\.subnets/p.Subnets/g' {} \;

# Update field references in structs
find . -name "*.go" -exec sed -i '' 's/g\.forkDigest/g.forkDigest/g' {} \;
find . -name "*.go" -exec sed -i '' 's/g\.encoder/g.encoder/g' {} \;
find . -name "*.go" -exec sed -i '' 's/g\.log/g.log/g' {} \;
