package fetcher

import (
	"context"
	"slices"
	"sync/atomic"

	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
)

// Fetcher fetches remotes when asked to and publishes a Fetched event
// on the broker at the end of each fetch.
type Fetcher interface {
	Start(ctx context.Context)
	TriggerFetch(remotes []string)
	IsFetching() bool
	GetState() *protobuf.Fetcher
	// RemoteNames returns the name of all remotes managed by the fetcher
	RemoteNames() []string
}

// loop accumulates the remotes submitted on submitRemotes and calls
// fetch with them when no fetch is running. When a fetch is done, its
// result is passed to onFetched.
func loop[T any](ctx context.Context, submitRemotes chan []string, isFetching *atomic.Bool, fetch func(context.Context, []string) chan T, onFetched func(T)) {
	remotes := make([]string, 0)
	var resultCh chan T
	for {
		select {
		case submittedRemotes := <-submitRemotes:
			logrus.Debugf("fetch: remotes submitted: %s", submittedRemotes)
			remotes = union(remotes, submittedRemotes)
		case result := <-resultCh:
			isFetching.Store(false)
			onFetched(result)
		}
		if !isFetching.Load() && len(remotes) != 0 {
			isFetching.Store(true)
			resultCh = fetch(ctx, remotes)
			remotes = []string{}
		}
	}
}

func union(array1, array2 []string) []string {
	for _, e2 := range array2 {
		exist := slices.Contains(array1, e2)
		if !exist {
			array1 = append(array1, e2)
		}
	}
	return array1
}
