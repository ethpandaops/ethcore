package eth

type GoodbyeReason uint8

const (
	GoodbyeReasonClientShutdown    GoodbyeReason = 0
	GoodbyeReasonIrrelevantNetwork GoodbyeReason = 1
	GoodbyeReasonFaultError        GoodbyeReason = 2
)
