package store

// Buckets define BoltDB schema.
const (
	BucketWorkspace = "workspace"
	BucketCache     = "cache"
	BucketAudit     = "audit"
	BucketSessions  = "sessions"
	BucketSnapshots = "snapshots"
)

var AllBuckets = [][]byte{
	[]byte(BucketWorkspace),
	[]byte(BucketCache),
	[]byte(BucketAudit),
	[]byte(BucketSessions),
	[]byte(BucketSnapshots),
}
