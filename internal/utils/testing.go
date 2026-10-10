package utils

import (
	"context"

	"github.com/nlewo/comin/pkg/protobuf"
)

type RepositoryMock struct {
	RsCh chan *protobuf.GitRepositoryStatus
}

func NewRepositoryMock() (r *RepositoryMock) {
	rsCh := make(chan *protobuf.GitRepositoryStatus, 5)
	return &RepositoryMock{
		RsCh: rsCh,
	}
}
func (r *RepositoryMock) FetchAndUpdate(ctx context.Context, remoteNames []string) (rsCh chan *protobuf.GitRepositoryStatus) {
	return r.RsCh
}
func (r *RepositoryMock) GetRepositoryStatus() *protobuf.GitRepositoryStatus {
	return &protobuf.GitRepositoryStatus{}
}

type OCIRepositoryMock struct {
	RsCh chan *protobuf.OCIRepositoryStatus
}

func NewOCIRepositoryMock() *OCIRepositoryMock {
	return &OCIRepositoryMock{
		RsCh: make(chan *protobuf.OCIRepositoryStatus, 5),
	}
}
func (r *OCIRepositoryMock) FetchAndUpdate(ctx context.Context, remoteNames []string) chan *protobuf.OCIRepositoryStatus {
	return r.RsCh
}
func (r *OCIRepositoryMock) GetRepositoryStatus() *protobuf.OCIRepositoryStatus {
	return &protobuf.OCIRepositoryStatus{
		Remotes: []*protobuf.OCIRemote{{Name: "ghcr"}},
	}
}
