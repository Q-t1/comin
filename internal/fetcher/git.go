package fetcher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type GitFetcher struct {
	isFetching       atomic.Bool
	repositoryStatus *protobuf.GitRepositoryStatus
	mu               sync.RWMutex
	submitRemotes    chan []string
	repo             repository.Repository
	broker           *broker.Broker
	remoteNames      []string
}

func NewGitFetcher(repo repository.Repository, broker *broker.Broker) *GitFetcher {
	f := &GitFetcher{
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

func (f *GitFetcher) IsFetching() bool {
	return f.isFetching.Load()
}

func (f *GitFetcher) TriggerFetch(remotes []string) {
	f.submitRemotes <- remotes
}

func (f *GitFetcher) RemoteNames() []string {
	return f.remoteNames
}

type RemoteState struct {
	Name      string    `json:"name"`
	FetchedAt time.Time `json:"fetched_at"`
}

type State struct {
	IsFetching       bool
	RepositoryStatus *protobuf.GitRepositoryStatus
}

func (f *GitFetcher) GetState() *protobuf.Fetcher {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return &protobuf.Fetcher{
		IsFetching: wrapperspb.Bool(f.isFetching.Load()),
		Status: &protobuf.Fetcher_GitRepositoryStatus{
			GitRepositoryStatus: f.repo.GetRepositoryStatus(),
		},
	}
}

func (f *GitFetcher) Start(ctx context.Context) {
	logrus.Info("fetcher git: starting")
	go loop(ctx, f.submitRemotes, &f.isFetching, f.repo.FetchAndUpdate, func(rs *protobuf.GitRepositoryStatus) {
		f.mu.Lock()
		updated := rs.SelectedCommitId != f.repositoryStatus.SelectedCommitId ||
			(rs.SelectedBranchIsTesting != nil &&
				rs.SelectedBranchIsTesting.GetValue() != f.repositoryStatus.SelectedBranchIsTesting.GetValue())
		if updated {
			f.repositoryStatus = rs
		}
		f.mu.Unlock()
		// Check if the commit is verified (signed when it should be)
		verified := !rs.SelectedCommitShouldBeSigned.GetValue() || rs.SelectedCommitSigned.GetValue()
		f.broker.Publish(&protobuf.Event{Type: &protobuf.Event_Fetched_{Fetched: &protobuf.Event_Fetched{Type: &protobuf.Event_Fetched_GitRepositoryStatus{GitRepositoryStatus: rs}, Updated: updated, Verified: verified}}, CreatedAt: timestamppb.New(time.Now().UTC())})
	})
}
