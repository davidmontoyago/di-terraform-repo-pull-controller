package poller

import (
	"fmt"
	"time"
)

const POLLING_FREQUENCY_SECONDS = 30

type RepoPoller struct {
	RepoKey string
	Ticker  *time.Ticker
	Done    chan bool
}

func NewRepoPoller(repoKey string) *RepoPoller {
	ticker := time.NewTicker(POLLING_FREQUENCY_SECONDS * time.Second)
	done := make(chan bool)
	return &RepoPoller{RepoKey: repoKey, Ticker: ticker, Done: done}
}

func (poller *RepoPoller) Start() {
	go func() {
		for {
			select {
			case <-poller.Done:
				return
			case t := <-poller.Ticker.C:
				// TODO check git repo for new HEADs
				fmt.Println("Checking for repo changes at", t)
			}
		}
	}()
}

func (poller *RepoPoller) Stop() {
	poller.Ticker.Stop()
	poller.Done <- true
}
