package neofs

import (
	"time"
)

// Constants related to NeoFS block storage.
const (
	// DefaultTimeout is the default timeout for NeoFS requests.
	DefaultTimeout = 10 * time.Minute
	// DefaultDownloaderWorkersCount is the default number of workers downloading blocks.
	DefaultDownloaderWorkersCount = 500
	// DefaultIndexFileSize is the default size of the index file.
	DefaultIndexFileSize = 128000
	// DefaultBlockAttribute is the default attribute name for block objects.
	DefaultBlockAttribute = "Block"
	// DefaultIndexFileAttribute is the default attribute name for index file objects.
	DefaultIndexFileAttribute = "Index"

	// DefaultSearchBatchSize is a number of objects to search in a batch. We need to
	// search with EQ filter to avoid partially-completed SEARCH responses. If EQ search
	// hasn't found object the object will be uploaded one more time which may lead to
	// duplicating objects. We will have a risk of duplicates until #3645 is resolved
	// (NeoFS guarantees search results).
	DefaultSearchBatchSize = 1
)

// Constants related to NeoFS pool request timeouts.
const (
	// DefaultDialTimeout is a default timeout used to establish connection with
	// NeoFS storage nodes.
	DefaultDialTimeout = 30 * time.Second
	// DefaultStreamTimeout is a default timeout used for NeoFS streams processing.
	// It has significantly large value to reliably avoid timeout problems with heavy
	// SEARCH requests.
	DefaultStreamTimeout = 10 * time.Minute
	// DefaultHealthcheckTimeout is a timeout for request to NeoFS storage node to
	// decide if it is alive.
	DefaultHealthcheckTimeout = 10 * time.Second
)

// Constants related to retry mechanism.
const (
	// MaxRetries is the maximum number of retries for a single operation.
	MaxRetries = 5
	// InitialBackoff is the initial backoff duration.
	InitialBackoff = 500 * time.Millisecond
	// BackoffFactor is the factor by which the backoff duration is multiplied.
	BackoffFactor = 2
	// MaxBackoff is the maximum backoff duration.
	MaxBackoff = 20 * time.Second
)
