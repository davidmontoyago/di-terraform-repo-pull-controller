package poller

import (
	"log"
	"time"

	repo "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"k8s.io/klog"
)

const POLLING_FREQUENCY_SECONDS = 30

type RepoPoller struct {
	RepoKey string
	Repo    repo.Repo
	Ticker  *time.Ticker
	Done    chan bool
}

func NewRepoPoller(repoKey string, repo repo.Repo) *RepoPoller {
	ticker := time.NewTicker(POLLING_FREQUENCY_SECONDS * time.Second)
	done := make(chan bool)
	return &RepoPoller{RepoKey: repoKey, Repo: repo, Ticker: ticker, Done: done}
}

func (poller *RepoPoller) Start() {
	go func() {
		for {
			select {
			case <-poller.Done:
				return
			case t := <-poller.Ticker.C:
				klog.Infof("Checking for repo changes at %s", t)
				poller.CheckForNewRevisions()
			}
		}
	}()
}

func (poller *RepoPoller) Stop() {
	poller.Ticker.Stop()
	poller.Done <- true
}

func (poller *RepoPoller) CheckForNewRevisions() {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{poller.Repo.Spec.Url},
	})

	klog.Infof("Checking for new revisions at %s...", poller.Repo.Spec.Url)
	refs, err := rem.List(&git.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	masterRef := FindMasterRef(refs)
	masterHash := masterRef.Hash().String()
	klog.Infof("Repo HEAD is %s", masterHash)
	lastScheduledRef := poller.Repo.Status.LastGitRef
	if masterHash != lastScheduledRef {
		klog.Infof("Found new commit reference %s... Previous was %s", masterHash, lastScheduledRef)
	}
}

func FindMasterRef(refs []*plumbing.Reference) plumbing.Reference {
	var master plumbing.Reference
	for _, ref := range refs {
		if ref.Name() == plumbing.Master {
			klog.Infof("Ref %+v", ref)
			master = *ref
			break
		}
	}
	return master
}
