
Locked-in architecture (v1, single cluster)

-Agent: DaemonSet, one per node, watches its own node's pods via local kubelet/informer, scrapes anything matching a shared label-based policy on its own schedule.
-Scheduled mode: policy lives centrally (CRD or ConfigMap the API server manages), agents pull/watch and reconcile independently. No broker.
-Triggered mode: API server resolves target (service/deployment) → pod(s) → node(s) via the k8s API, sends a direct call to each relevant agent. No broadcast.
-Storage: needs a time-range-indexed, compaction-aware format to support weeks+ retention without unbounded growth — this is still an open design piece.
-API server: owns policy config, trigger resolution, and query/diff endpoints. CLI and CI/CD are both just clients of it.


Storage:

blobs poc - badger, v1 - add s3/minio support
index - portable sql with poc - sqlite, v1 - add postgres support, use versioned migrations

For both of these we will use adapter pattern
