package protobuf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestSourceAccessorsNil(t *testing.T) {
	var s *Source
	assert.Equal(t, "", s.RemoteName())
	assert.Equal(t, "", s.RefName())
	assert.Equal(t, "", s.Id())
	assert.Equal(t, "", s.Message())
	assert.Equal(t, "", s.Hostname())
	assert.False(t, s.IsTesting())

	var g *Generation
	assert.Equal(t, "", g.GetSource().Id())
}

func TestSourceAccessorsGit(t *testing.T) {
	s := &Source{
		Source: &Source_Git{
			Git: &Git{
				Hostname:                "machine",
				SelectedRemoteName:      "origin",
				SelectedBranchName:      "testing",
				SelectedCommitId:        "abcd",
				SelectedCommitMsg:       "msg\n",
				SelectedBranchIsTesting: wrapperspb.Bool(true),
			},
		},
	}
	assert.Equal(t, "origin", s.RemoteName())
	assert.Equal(t, "testing", s.RefName())
	assert.Equal(t, "abcd", s.Id())
	assert.Equal(t, "msg", s.Message())
	assert.Equal(t, "machine", s.Hostname())
	assert.True(t, s.IsTesting())
}

func TestSourceAccessorsOCI(t *testing.T) {
	s := &Source{
		Source: &Source_Oci{
			Oci: &OCI{
				Hostname:           "machine",
				SelectedRemoteName: "ghcr",
				SelectedTag:        "1.2.3",
				SelectedDigest:     "sha256:abcd",
				SelectedRevision:   "main@sha1:1234",
				SelectedIsTesting:  wrapperspb.Bool(false),
			},
		},
	}
	assert.Equal(t, "ghcr", s.RemoteName())
	assert.Equal(t, "1.2.3", s.RefName())
	assert.Equal(t, "sha256:abcd", s.Id())
	assert.Equal(t, "main@sha1:1234", s.Message())
	assert.Equal(t, "machine", s.Hostname())
	assert.False(t, s.IsTesting())

	// An artifact pinned by digest has no tag
	s.GetOci().SelectedTag = ""
	assert.Equal(t, "main", s.RefName())
	s.GetOci().SelectedIsTesting = wrapperspb.Bool(true)
	assert.Equal(t, "testing", s.RefName())
	assert.True(t, s.IsTesting())
}
