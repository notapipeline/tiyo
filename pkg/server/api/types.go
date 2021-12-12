package api

import "sync"

type simple struct {
	Value string `json:"value"`
}

// GithubResponse : Expected information from the Github request
type GithubResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Result : A server result defines the body wrapper sent back to the client
type Result struct {

	// HTTP response code
	Code int `json:"code"`

	// HTTP Result string - normally one of OK | Error
	Result string `json:"result"`

	// The request message being returned
	Message interface{} `json:"message"`
}

// ScanResult : Return values stored in a bucket
type ScanResult struct {

	// Buckets is a list of child buckets the scanned bucket might contain
	Buckets []string `json:"buckets"`

	// For simplicity on the caller side, send the number of child buckets found
	BucketsLength int `json:"bucketlen"`

	// A map of key:value pairs containing bucket data
	Keys map[string]string `json:"keys"`

	// For simplicity, the number of keys in the bucket
	KeyLen int `json:"keylen"`
}

// Lock : Allow for locking individual queues whilst manipulating them
type Lock struct {
	sync.Mutex
	locks []string
}
