package config

type IndexProviderConfig struct {
	// Enable set whether to enable indexing announcement to the network and expose endpoints that
	// allow indexer nodes to process announcements. Enabled by default.
	Enable bool

	// EntriesCacheCapacity sets the maximum capacity to use for caching the indexing advertisement
	// entries. Defaults to 1024 if not specified. The cache is evicted using LRU policy. The
	// maximum storage used by the cache is a factor of EntriesCacheCapacity, EntriesChunkSize and
	// the length of multihashes being advertised. For example, advertising 128-bit long multihashes
	// with the default EntriesCacheCapacity, and EntriesChunkSize means the cache size can grow to
	// 256MiB when full.
	EntriesCacheCapacity int

	// EntriesChunkSize sets the maximum number of multihashes to include in a single entries chunk.
	// Defaults to 16384 if not specified. Note that chunks are chained together for indexing
	// advertisements that include more multihashes than the configured EntriesChunkSize.
	EntriesChunkSize int

	// TopicName sets the topic name on which the changes to the advertised content are announced.
	// If not explicitly specified, the topic name is automatically inferred from the network name
	// in following format: '/indexer/ingest/<network-name>'
	// Defaults to empty, which implies the topic name is inferred from network name.
	TopicName string

	// PurgeCacheOnStart sets whether to clear any cached entries chunks when the provider engine
	// starts. By default, the cache is rehydrated from previously cached entries stored in
	// datastore if any is present.
	PurgeCacheOnStart bool
}
