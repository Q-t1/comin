package fetcher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/oci"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type OCIFetcher struct {
	isFetching       atomic.Bool
	repositoryStatus *protobuf.OCIRepositoryStatus
	mu               sync.RWMutex
	submitRemotes    chan []string
	repo             oci.Repository
	broker           *broker.Broker
	remoteNames      []string
}

func NewOCIFetcher(repo oci.Repository, broker *broker.Broker) *OCIFetcher {
	f := &OCIFetcher{
		repo:          repo,
		broker:        broker,
		submitRemotes: make(chan []string),
	}
	f.repositoryStatus = repo.GetRepositoryStatus()
	for _, r := range f.repositoryStatus.Remotes {
		f.remoteNames = append(f.remoteNames, r.Name)
	}
	return f
}

func (f *OCIFetcher) IsFetching() bool {
	return f.isFetching.Load()
}

func (f *OCIFetcher) TriggerFetch(remotes []string) {
	f.submitRemotes <- remotes
}

func (f *OCIFetcher) RemoteNames() []string {
	return f.remoteNames
}

func (f *OCIFetcher) GetState() *protobuf.Fetcher {
	return &protobuf.Fetcher{
		IsFetching: wrapperspb.Bool(f.isFetching.Load()),
		Status: &protobuf.Fetcher_OciRepositoryStatus{
			OciRepositoryStatus: f.repo.GetRepositoryStatus(),
		},
	}
}

func (f *OCIFetcher) Start(ctx context.Context) {
	logrus.Info("fetcher oci: starting")
	go loop(ctx, f.submitRemotes, &f.isFetching, f.repo.FetchAndUpdate, func(rs *protobuf.OCIRepositoryStatus) {
		f.mu.Lock()
		updated := rs.SelectedDigest != f.repositoryStatus.SelectedDigest ||
			(rs.SelectedIsTesting != nil &&
				rs.SelectedIsTesting.GetValue() != f.repositoryStatus.SelectedIsTesting.GetValue())
		if updated {
			f.repositoryStatus = rs
		}
		f.mu.Unlock()
		verified := !rs.SelectedShouldBeSigned.GetValue() || rs.SelectedSigned.GetValue()
		f.broker.Publish(&protobuf.Event{Type: &protobuf.Event_Fetched_{Fetched: &protobuf.Event_Fetched{Type: &protobuf.Event_Fetched_OciRepositoryStatus{OciRepositoryStatus: rs}, Updated: updated, Verified: verified}}, CreatedAt: timestamppb.New(time.Now().UTC())})
	})
}
