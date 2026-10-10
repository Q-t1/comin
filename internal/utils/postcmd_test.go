package utils

import (
	"testing"

	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestRunPostCommand(t *testing.T) {
	g := &pb.Generation{Uuid: "uuid"}

	out, err := RunPostCommand("env", "build", g, "built", "")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_PHASE=build")
	assert.Contains(t, out, "COMIN_GIT_SHA=")
	assert.Contains(t, out, "COMIN_GENERATION=uuid")
	assert.Contains(t, out, "COMIN_STATUS=built")

	out, err = RunPostCommand("env", "deploy", g, "done", "")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_PHASE=deploy")
	assert.Contains(t, out, "COMIN_STATUS=done")
}

func TestRunPostCommandExtras(t *testing.T) {
	g := &pb.Generation{Uuid: "uuid"}

	out, err := RunPostCommand("env", "build", g, "built", "", "COMIN_DRV_PATH=/nix/store/abc")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_DRV_PATH=/nix/store/abc")
}

func TestRunPostCommandOCI(t *testing.T) {
	g := &pb.Generation{
		Uuid: "uuid",
		Source: &pb.Source{
			Source: &pb.Source_Oci{
				Oci: &pb.OCI{
					Hostname:           "machine",
					SelectedRemoteName: "ghcr",
					SelectedTag:        "latest",
					SelectedDigest:     "sha256:abcd",
					SelectedRevision:   "main@sha1:1234",
				},
			},
		},
	}
	out, err := RunPostCommand("env", "deploy", g, "done", "")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_GIT_SHA=1234\n")
	assert.Contains(t, out, "COMIN_GIT_REF=main\n")
	assert.Contains(t, out, "COMIN_HOSTNAME=machine\n")
	assert.Contains(t, out, "COMIN_OCI_REMOTE=ghcr\n")
	assert.Contains(t, out, "COMIN_OCI_TAG=latest\n")
	assert.Contains(t, out, "COMIN_OCI_DIGEST=sha256:abcd\n")
}

func TestParseRevision(t *testing.T) {
	ref, id := parseRevision("main@sha1:1234")
	assert.Equal(t, "main", ref)
	assert.Equal(t, "1234", id)
	ref, id = parseRevision("sha1:1234")
	assert.Equal(t, "", ref)
	assert.Equal(t, "1234", id)
	ref, id = parseRevision("1234")
	assert.Equal(t, "", ref)
	assert.Equal(t, "1234", id)
}
