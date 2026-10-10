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
