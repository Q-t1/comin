package protobuf

import (
	"fmt"
	"strings"
)

// The following accessors expose the attributes shared by all source
// types. They are nil safe and return zero values when the source is
// not set.

// RemoteName returns the name of the remote the source has been fetched from.
func (s *Source) RemoteName() string {
	switch {
	case s.GetGit() != nil:
		return s.GetGit().SelectedRemoteName
	case s.GetOci() != nil:
		return s.GetOci().SelectedRemoteName
	}
	return ""
}

// RefName returns the name of the reference the source has been
// selected from: the Git branch or the OCI tag. When an OCI artifact
// is pinned by digest, this is "main" or "testing".
func (s *Source) RefName() string {
	switch {
	case s.GetGit() != nil:
		return s.GetGit().SelectedBranchName
	case s.GetOci() != nil:
		if s.GetOci().SelectedTag != "" {
			return s.GetOci().SelectedTag
		}
		if s.IsTesting() {
			return "testing"
		}
		return "main"
	}
	return ""
}

// Id returns the identifier of the source content: the Git commit ID
// or the OCI manifest digest.
func (s *Source) Id() string {
	switch {
	case s.GetGit() != nil:
		return s.GetGit().SelectedCommitId
	case s.GetOci() != nil:
		return s.GetOci().SelectedDigest
	}
	return ""
}

// Message returns a message describing the source content: the Git
// commit message or the revision the OCI artifact has been built from.
func (s *Source) Message() string {
	switch {
	case s.GetGit() != nil:
		return strings.Trim(s.GetGit().SelectedCommitMsg, "\n")
	case s.GetOci() != nil:
		return s.GetOci().SelectedRevision
	}
	return ""
}

// IsTesting returns true if the source has been selected from a
// testing reference.
func (s *Source) IsTesting() bool {
	switch {
	case s.GetGit() != nil:
		return s.GetGit().SelectedBranchIsTesting.GetValue()
	case s.GetOci() != nil:
		return s.GetOci().SelectedIsTesting.GetValue()
	}
	return false
}

// Hostname returns the hostname of the configuration to deploy.
func (s *Source) Hostname() string {
	switch {
	case s.GetGit() != nil:
		return s.GetGit().Hostname
	case s.GetOci() != nil:
		return s.GetOci().Hostname
	}
	return ""
}

// Short return a short line for each logs.
// This function is experimental and subject to change.
func (event *Event) Short() string {
	var eventType, payload string
	switch t := event.Type.(type) {
	case *Event_EvalStartedType:
		eventType = "eval-started   "
		g := t.EvalStartedType.Generation
		src := g.GetSource()
		payload = fmt.Sprintf("gen-uuid=%s ref-id=%s/%s/%s", g.Uuid, src.RemoteName(), src.RefName(), src.Id())
	case *Event_EvalFinishedType:
		eventType = "eval-finished  "
		g := t.EvalFinishedType.Generation
		src := g.GetSource()
		payload = fmt.Sprintf("gen-uuid=%s ref-id=%s/%s/%s status=%s", g.Uuid, src.RemoteName(), src.RefName(), src.Id(), g.EvalStatus)
	case *Event_BuildStartedType:
		eventType = "build-started  "
		g := t.BuildStartedType.Generation
		src := g.GetSource()
		payload = fmt.Sprintf("gen-uuid=%s ref-id=%s/%s/%s", g.Uuid, src.RemoteName(), src.RefName(), src.Id())
	case *Event_BuildFinishedType:
		eventType = "build-finished "
		g := t.BuildFinishedType.Generation
		src := g.GetSource()
		payload = fmt.Sprintf("gen-uuid=%s ref-id=%s/%s/%s status=%s", g.Uuid, src.RemoteName(), src.RefName(), src.Id(), g.BuildStatus)
	case *Event_ConfirmationSubmittedType:
		eventType = "cf-submitted   "
	case *Event_ConfirmationCancelledType:
		eventType = "cf-cancelled   "
	case *Event_ConfirmationConfirmedType:
		eventType = "cf-confirmed   "
	case *Event_Resume_:
		eventType = "resume         "
	case *Event_Suspend_:
		eventType = "suspend        "
	case *Event_DeploymentStartedType:
		eventType = "dpl-started    "
		g := t.DeploymentStartedType.Deployment.Generation
		d := t.DeploymentStartedType.Deployment
		src := g.GetSource()
		payload = fmt.Sprintf("gen-uuid=%s ref-id=%s/%s/%s op=%s", g.Uuid, src.RemoteName(), src.RefName(), src.Id(), d.Operation)
	case *Event_DeploymentFinishedType:
		eventType = "dpl-finished   "
		g := t.DeploymentFinishedType.Deployment.Generation
		d := t.DeploymentFinishedType.Deployment
		src := g.GetSource()
		payload = fmt.Sprintf("gen-uuid=%s ref-id=%s/%s/%s op=%s status=%s", g.Uuid, src.RemoteName(), src.RefName(), src.Id(), d.Operation, d.Status)
	case *Event_RebootRequired_:
		eventType = "reboot-required"
	case *Event_ManagerState_:
		eventType = "manager-state  "
	case *Event_Fetched_:
		eventType = "fetched        "
	case *Event_Log_:
		eventType = "log            "
		switch l := t.Log.Type.(type) {
		case *Event_Log_Open_:
			payload = "open"
		case *Event_Log_Close_:
			payload = "close"
		case *Event_Log_Line_:
			payload = fmt.Sprintf("src=%s msg=%s", l.Line.Source, l.Line.Msg)
		}
	default:
		eventType = "unknown"
	}
	return fmt.Sprintf("%s %s", eventType, payload)
}
