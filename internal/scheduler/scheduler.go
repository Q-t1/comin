package scheduler

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/types"
	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	s gocron.Scheduler
}

func New() Scheduler {
	s, _ := gocron.NewScheduler()

	sched := Scheduler{
		s: s,
	}
	go sched.s.Start()
	return sched
}

// FetchRemote periodically triggers a fetch of the remote remoteName
// if the poller period is not 0.
func (s Scheduler) FetchRemote(fetcher fetcher.Fetcher, remoteName string, poller types.Poller) {
	if poller.Period != 0 {
		logrus.Infof("scheduler: starting the period job for the remote '%s' with period %ds", remoteName, poller.Period)
		_, _ = s.s.NewJob(
			gocron.DurationJob(
				time.Duration(poller.Period)*time.Second,
			),
			gocron.NewTask(
				func() {
					logrus.Debugf("scheduler: running task for remote %s", remoteName)
					fetcher.TriggerFetch([]string{remoteName})
				},
			),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
			gocron.WithName(fmt.Sprintf("fetch-remote-%s", remoteName)),
		)
	}
}
