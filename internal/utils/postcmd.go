package utils

import (
	"fmt"
	"os"
	"os/exec"

	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
)

// RunPostCommand runs command as a post-build or post-deployment hook,
// exporting the standard comin environment variables and any phase-specific
// extras passed by the caller.
//
// phase is exported as COMIN_PHASE ("build" or "deploy"). status and errMsg
// are exported as COMIN_STATUS and COMIN_ERROR_MSG; the caller owns their
// meaning (e.g. build uses "built"/"failed"/"already built", deploy uses
// "done"/"failed"). extra is a list of "KEY=VALUE" entries appended last
// so phase-specific vars (COMIN_DRV_PATH, COMIN_PROFILE_PATH, ...) can be
// supplied without changing the common signature.
func RunPostCommand(command, phase string, g *pb.Generation, status, errMsg string, extra ...string) (string, error) {
	cmd := exec.Command(command)
	cmd.Env = append(os.Environ(),
		"COMIN_PHASE="+phase,
		"COMIN_GIT_SHA="+gitSha(g),
		"COMIN_GIT_REF="+gitRef(g),
		"COMIN_GIT_MSG="+gitMessage(g),
		"COMIN_HOSTNAME="+cominHostname(g),
		"COMIN_GENERATION="+cominGeneration(g),
		"COMIN_STATUS="+status,
		"COMIN_ERROR_MSG="+errMsg,
	)
	cmd.Env = append(cmd.Env, extra...)

	output, err := cmd.CombinedOutput()
	outputString := string(output)
	if err != nil {
		return outputString, err
	}
	logrus.Debugf("cmd:[%s] output:[%s]", command, outputString)
	return outputString, nil
}

func gitSha(g *pb.Generation) string {
	return g.GetSource().Id()
}

func gitRef(g *pb.Generation) string {
	if g.GetSource() == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", g.GetSource().RemoteName(), g.GetSource().RefName())
}

func gitMessage(g *pb.Generation) string {
	return g.GetSource().Message()
}

func cominHostname(g *pb.Generation) string {
	return g.GetSource().Hostname()
}

func cominGeneration(g *pb.Generation) string {
	return g.Uuid
}
