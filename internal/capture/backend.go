package capture

const (
	BackendRawSocket = "raw_socket"
	BackendPktmon    = "pktmon"
	BackendNpcap     = "npcap"
)

const (
	LimitCircularBuffer    = "circular_buffer_may_overwrite_oldest"
	LimitTruncated         = "packets_truncated"
	LimitComponentIdentity = "component_identity_lost"
	LimitAllNICs           = "capture_scope_all_nics"
	LimitDuplicates        = "duplicates_possible"
	LimitDropCountMissing  = "drop_count_unavailable"
)
