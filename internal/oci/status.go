package oci

import (
	"fmt"

	"github.com/nlewo/comin/internal/types"
	pb "github.com/nlewo/comin/pkg/protobuf"
)

// MainArtifact describes the last deployed main artifact. It is used
// to refuse artifacts older than this one.
type MainArtifact struct {
	Digest     string
	Tag        string
	Created    string
	RemoteName string
}

func NewOCIRepositoryStatus(config types.OCIConfig, main MainArtifact) *pb.OCIRepositoryStatus {
	s := &pb.OCIRepositoryStatus{
		MainDigest:     main.Digest,
		MainTag:        main.Tag,
		MainCreated:    main.Created,
		MainRemoteName: main.RemoteName,
	}
	s.Remotes = make([]*pb.OCIRemote, len(config.Remotes))
	for i, remote := range config.Remotes {
		s.Remotes[i] = &pb.OCIRemote{
			Name:    remote.Name,
			Url:     remote.URL,
			Main:    &pb.OCIReference{Spec: refSpec(remote.Ref.Main)},
			Testing: &pb.OCIReference{Spec: refSpec(remote.Ref.Testing)},
		}
	}
	return s
}

// refSpec returns a human readable representation of a reference
func refSpec(ref types.OCIRef) string {
	switch {
	case ref.Digest != "":
		return "digest:" + ref.Digest
	case ref.Semver != "" && ref.SemverFilter != "":
		return fmt.Sprintf("semver:%s (filter: %s)", ref.Semver, ref.SemverFilter)
	case ref.Semver != "":
		return "semver:" + ref.Semver
	case ref.Tag != "":
		return "tag:" + ref.Tag
	}
	return ""
}
