// Package oci fetches the configuration to deploy from OCI artifacts.
//
// It is the OCI counterpart of the repository package. Each remote is
// an OCI repository with a main and an optional testing reference.
// A reference is a tag, a semver constraint or a digest. On each
// fetch, references are resolved to artifacts and an artifact is
// selected to be deployed by following the Git commit selection
// algorithm:
//
//  1. the first main artifact which differs from the deployed main
//     artifact is selected,
//  2. a testing artifact differing from the selected main artifact is
//     then preferred.
//
// Since there is no Git history to rely on, artifacts older than the
// deployed main artifact are refused: an artifact is older when its
// semver tag is lower (for semver references) or when its
// org.opencontainers.image.created annotation is older.
//
// Once selected, the artifact is pulled by digest and its layer is
// extracted into a directory named by the artifact digest. If cosign
// public keys are configured, the artifact is only selected if it is
// signed by one of these keys.
package oci

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/nlewo/comin/internal/types"
	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Repository interface {
	FetchAndUpdate(ctx context.Context, remoteNames []string) chan *pb.OCIRepositoryStatus
	GetRepositoryStatus() *pb.OCIRepositoryStatus
}

type repository struct {
	config types.OCIConfig
	// verifier is nil when no cosign public keys are configured
	verifier *cosignVerifier
	// mu protects status, which is replaced at the end of each fetch
	mu     sync.Mutex
	status *pb.OCIRepositoryStatus
}

// New creates a repository. The main artifact is the last deployed
// main artifact: older artifacts are refused.
func New(config types.OCIConfig, main MainArtifact) (*repository, error) {
	r := &repository{
		config: config,
		status: NewOCIRepositoryStatus(config, main),
	}
	if len(config.CosignPublicKeyPaths) > 0 {
		verifier, err := newCosignVerifier(config.CosignPublicKeyPaths)
		if err != nil {
			return nil, err
		}
		r.verifier = verifier
	}
	return r, nil
}

func (r *repository) GetRepositoryStatus() *pb.OCIRepositoryStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return proto.CloneOf(r.status)
}

func (r *repository) FetchAndUpdate(ctx context.Context, remoteNames []string) chan *pb.OCIRepositoryStatus {
	ch := make(chan *pb.OCIRepositoryStatus)
	go func() {
		status := r.GetRepositoryStatus()
		r.fetch(ctx, status, remoteNames)
		r.update(ctx, status)
		r.mu.Lock()
		r.status = status
		r.mu.Unlock()
		ch <- proto.CloneOf(status)
	}()
	return ch
}

func (r *repository) remoteConfig(name string) (types.OCIRemote, bool) {
	for _, remote := range r.config.Remotes {
		if remote.Name == name {
			return remote, true
		}
	}
	return types.OCIRemote{}, false
}

// fetch resolves the references of the remotes remoteNames
func (r *repository) fetch(ctx context.Context, status *pb.OCIRepositoryStatus, remoteNames []string) {
	logrus.Debugf("oci: fetching %s", remoteNames)
	for i, remoteConfig := range r.config.Remotes {
		if !slices.Contains(remoteNames, remoteConfig.Name) {
			continue
		}
		remote := status.Remotes[i]
		if err := fetchRemote(ctx, remoteConfig, remote); err != nil {
			logrus.Errorf("oci: failed to fetch the remote '%s': %s", remoteConfig.Name, err)
			remote.FetchErrorMsg = err.Error()
			remote.Fetched = wrapperspb.Bool(false)
		} else {
			remote.FetchErrorMsg = ""
			remote.Fetched = wrapperspb.Bool(true)
		}
		remote.FetchedAt = timestamppb.New(time.Now().UTC())
	}
}

// fetchRemote resolves the main and testing references of a remote. A
// reference which doesn't match any artifact is not a fetch error:
// its error is stored in the reference status.
func fetchRemote(ctx context.Context, remoteConfig types.OCIRemote, remote *pb.OCIRemote) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(remoteConfig.Timeout)*time.Second)
	defer cancel()
	repo, err := repositoryName(remoteConfig)
	if err != nil {
		return err
	}
	opts, err := remoteOptions(ctx, remoteConfig, repo)
	if err != nil {
		return err
	}
	if err := resolveReference(repo, remoteConfig.Ref.Main, remote.Main, opts); err != nil {
		return err
	}
	if remoteConfig.Ref.Testing.IsDefined() {
		if err := resolveReference(repo, remoteConfig.Ref.Testing, remote.Testing, opts); err != nil {
			return err
		}
	}
	return nil
}

// resolveReference resolves ref and updates the reference status. It
// only returns an error when the registry fails.
func resolveReference(repo name.Repository, ref types.OCIRef, reference *pb.OCIReference, opts []remote.Option) error {
	previous := artifact{
		tag:       reference.Tag,
		digest:    reference.Digest,
		revision:  reference.Revision,
		sourceURL: reference.SourceUrl,
		created:   reference.Created,
	}
	a, err := resolve(repo, ref, previous, opts)
	if errors.Is(err, errNotFound) {
		reference.Tag = ""
		reference.Digest = ""
		reference.Revision = ""
		reference.SourceUrl = ""
		reference.Created = ""
		reference.ErrorMsg = err.Error()
		return nil
	} else if err != nil {
		return err
	}
	if a.digest != reference.Digest {
		logrus.Infof("oci: the reference %s of %s resolves to the new artifact %s", reference.Spec, repo, a.digest)
	}
	reference.Tag = a.tag
	reference.Digest = a.digest
	reference.Revision = a.revision
	reference.SourceUrl = a.sourceURL
	reference.Created = a.created
	reference.ErrorMsg = ""
	return nil
}

// update selects the artifact to deploy, pulls it and updates the
// status. If the artifact can not be verified or pulled, the selection
// is not updated, in order to retry on the next fetch.
func (r *repository) update(ctx context.Context, status *pb.OCIRepositoryStatus) {
	status.ErrorMsg = ""
	main := MainArtifact{
		Digest:     status.MainDigest,
		Tag:        status.MainTag,
		Created:    status.MainCreated,
		RemoteName: status.MainRemoteName,
	}
	newMain := main
	var selected *pb.OCIReference
	var selectedRemote string
	selectedIsTesting := false

	// We first walk on all main references in order to get a main
	// artifact. Testing references are then walked to get a testing
	// artifact newer than the main artifact.
	for i, remote := range status.Remotes {
		if remote.FetchErrorMsg != "" {
			logrus.Debugf("oci: the remote %s is skipped because of the fetch error: %s", remote.Name, remote.FetchErrorMsg)
			continue
		}
		ref := remote.Main
		if ref.Digest == "" {
			continue
		}
		if err := checkNotOlder(r.config.Remotes[i].Ref.Main, ref, main); err != nil {
			ref.ErrorMsg = err.Error()
			continue
		}
		ref.ErrorMsg = ""
		if selected == nil {
			selected = ref
			selectedRemote = remote.Name
		}
		if ref.Digest != main.Digest {
			selected = ref
			selectedRemote = remote.Name
			newMain = MainArtifact{
				Digest:     ref.Digest,
				Tag:        ref.Tag,
				Created:    ref.Created,
				RemoteName: remote.Name,
			}
			break
		}
	}

	for i, remote := range status.Remotes {
		if remote.FetchErrorMsg != "" || remote.Testing.Spec == "" {
			continue
		}
		ref := remote.Testing
		if ref.Digest == "" {
			continue
		}
		if err := checkNotOlder(r.config.Remotes[i].Ref.Testing, ref, newMain); err != nil {
			ref.ErrorMsg = err.Error()
			continue
		}
		ref.ErrorMsg = ""
		if (selected == nil || ref.Digest != selected.Digest) && ref.Digest != newMain.Digest {
			selected = ref
			selectedRemote = remote.Name
			selectedIsTesting = true
			break
		}
	}

	if selected == nil {
		return
	}

	signedBy := status.SelectedSignedBy
	if selected.Digest != status.SelectedDigest {
		remoteConfig, _ := r.remoteConfig(selectedRemote)
		var err error
		if signedBy, err = r.verifyAndPull(ctx, remoteConfig, selected.Digest); err != nil {
			logrus.Errorf("oci: %s", err)
			status.ErrorMsg = err.Error()
			return
		}
	}

	previousSelectedDigest := status.SelectedDigest
	status.SelectedDigest = selected.Digest
	status.SelectedTag = selected.Tag
	status.SelectedRemoteName = selectedRemote
	status.SelectedIsTesting = wrapperspb.Bool(selectedIsTesting)
	status.SelectedRevision = selected.Revision
	status.SelectedSourceUrl = selected.SourceUrl
	status.SelectedCreated = selected.Created
	status.SelectedShouldBeSigned = wrapperspb.Bool(r.verifier != nil)
	status.SelectedSigned = wrapperspb.Bool(signedBy != "")
	status.SelectedSignedBy = signedBy
	status.MainDigest = newMain.Digest
	status.MainTag = newMain.Tag
	status.MainCreated = newMain.Created
	status.MainRemoteName = newMain.RemoteName

	// The previously selected artifact is kept because it could be
	// currently evaluated.
	gc(r.config.Path, []string{status.SelectedDigest, previousSelectedDigest, status.MainDigest})
}

// verifyAndPull verifies the artifact is signed when cosign public
// keys are configured, and then downloads and extracts it. It returns
// the public key used to verify the artifact signature.
func (r *repository) verifyAndPull(ctx context.Context, remoteConfig types.OCIRemote, digest string) (signedBy string, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(remoteConfig.Timeout)*time.Second)
	defer cancel()
	repo, err := repositoryName(remoteConfig)
	if err != nil {
		return "", err
	}
	opts, err := remoteOptions(ctx, remoteConfig, repo)
	if err != nil {
		return "", err
	}
	// The artifact is verified before being extracted in order to
	// not extract untrusted content.
	if r.verifier != nil {
		if signedBy, err = r.verifier.verify(repo, digest, opts); err != nil {
			return "", fmt.Errorf("the artifact %s is not signed by a trusted cosign key: %w", digest, err)
		}
	}
	if err := pull(r.config.Path, repo, digest, remoteConfig.LayerMediaType, opts); err != nil {
		return "", fmt.Errorf("failed to pull the artifact %s from %s: %w", digest, repo, err)
	}
	return signedBy, nil
}

// checkNotOlder returns an error if the candidate artifact is older
// than the main artifact. Since artifacts don't have a history, this
// is how rollbacks to older artifacts are refused. An artifact pinned
// by digest is never refused.
func checkNotOlder(ref types.OCIRef, candidate *pb.OCIReference, main MainArtifact) error {
	if ref.Digest != "" || main.Digest == "" || candidate.Digest == main.Digest {
		return nil
	}
	if ref.Semver != "" {
		cv, err1 := semver.NewVersion(candidate.Tag)
		mv, err2 := semver.NewVersion(main.Tag)
		if err1 == nil && err2 == nil {
			if cv.LessThan(mv) {
				return fmt.Errorf("the version %s is lower than the version %s of the main artifact %s", candidate.Tag, main.Tag, main.Digest)
			}
			if cv.GreaterThan(mv) {
				return nil
			}
		}
	}
	ct, err1 := time.Parse(time.RFC3339, candidate.Created)
	mt, err2 := time.Parse(time.RFC3339, main.Created)
	if err1 == nil && err2 == nil && ct.Before(mt) {
		return fmt.Errorf("the artifact %s has been created at %s, before the main artifact %s created at %s",
			candidate.Digest, candidate.Created, main.Digest, main.Created)
	}
	return nil
}
