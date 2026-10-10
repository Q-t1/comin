package fetcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/utils"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestFetcher(t *testing.T) {
	r := utils.NewRepositoryMock()
	bk := broker.New()
	bk.Start()
	f := NewGitFetcher(r, bk)
	f.Start(t.Context())

	// Subscribe to broker events
	brokerEvents := bk.Subscribe()

	var commitId string

	for i := range 2 {
		assert.False(t, f.IsFetching())
		f.TriggerFetch([]string{"remote"})

		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.True(c, f.IsFetching())
		}, 5*time.Second, 100*time.Millisecond, "fetcher is not fetching")

		// This is to simulate a git fetch
		commitId = fmt.Sprintf("id-%d", i)
		r.RsCh <- &protobuf.GitRepositoryStatus{
			SelectedCommitId: commitId,
		}
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			e := <-brokerEvents
			fetched := e.GetFetched()
			assert.NotNil(c, fetched)
			assert.True(c, fetched.Updated)
			assert.Equal(c, commitId, fetched.GetGitRepositoryStatus().SelectedCommitId)
		}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")

		assert.False(t, f.IsFetching())
	}

	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.GitRepositoryStatus{
		SelectedCommitId: "id-5",
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		e := <-brokerEvents
		fetched := e.GetFetched()
		assert.NotNil(c, fetched)
		assert.True(c, fetched.Updated)
		assert.Equal(c, "id-5", fetched.GetGitRepositoryStatus().SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")

	r.RsCh <- &protobuf.GitRepositoryStatus{
		SelectedCommitId: "id-5",
	}
	r.RsCh <- &protobuf.GitRepositoryStatus{
		SelectedCommitId: "id-6",
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		e := <-brokerEvents
		fetched := e.GetFetched()
		assert.NotNil(c, fetched)
		assert.True(c, fetched.Updated)
		assert.Equal(c, "id-6", fetched.GetGitRepositoryStatus().SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")
}

func TestUnion(t *testing.T) {
	res := union([]string{"r1", "r2"}, []string{"r1", "r3"})
	assert.Equal(t, []string{"r1", "r2", "r3"}, res)
}

func TestOCIFetcher(t *testing.T) {
	r := utils.NewOCIRepositoryMock()
	bk := broker.New()
	bk.Start()
	f := NewOCIFetcher(r, bk)
	f.Start(t.Context())
	brokerEvents := bk.Subscribe()
	assert.Equal(t, []string{"ghcr"}, f.RemoteNames())

	expect := func(digest string, updated bool) {
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			e := <-brokerEvents
			fetched := e.GetFetched()
			assert.NotNil(c, fetched)
			assert.Equal(c, updated, fetched.Updated)
			assert.True(c, fetched.Verified)
			assert.Equal(c, digest, fetched.GetOciRepositoryStatus().SelectedDigest)
		}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")
	}

	f.TriggerFetch([]string{"ghcr"})
	r.RsCh <- &protobuf.OCIRepositoryStatus{SelectedDigest: "sha256:1"}
	expect("sha256:1", true)

	f.TriggerFetch([]string{"ghcr"})
	r.RsCh <- &protobuf.OCIRepositoryStatus{SelectedDigest: "sha256:1"}
	expect("sha256:1", false)

	// The same artifact selected from a testing tag is an update
	f.TriggerFetch([]string{"ghcr"})
	r.RsCh <- &protobuf.OCIRepositoryStatus{SelectedDigest: "sha256:1", SelectedIsTesting: wrapperspb.Bool(true)}
	expect("sha256:1", true)

	assert.NotNil(t, f.GetState().GetOciRepositoryStatus())
}
