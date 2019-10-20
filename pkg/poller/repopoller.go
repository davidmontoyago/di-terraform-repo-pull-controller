package poller

import (
	"log"
	"time"

	"github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/status"
	repo "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"k8s.io/klog"
)

const POLLING_FREQUENCY_SECONDS = 30

type RepoPoller struct {
	RepoKey           string
	Repo              *repo.Repo
	Ticker            *time.Ticker
	Done              chan bool
	repoStatusManager status.RepoStatusManager
	gitRemote         GitRemote
}

func NewRepoPoller(repoKey string,
	repo *repo.Repo,
	repoStatusManager status.RepoStatusManager,
	gitRemote GitRemote) *RepoPoller {

	ticker := time.NewTicker(POLLING_FREQUENCY_SECONDS * time.Second)
	done := make(chan bool)

	return &RepoPoller{
		RepoKey:           repoKey,
		Repo:              repo,
		Ticker:            ticker,
		Done:              done,
		repoStatusManager: repoStatusManager,
		gitRemote:         gitRemote,
	}
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
	klog.Infof("Checking for new revisions at %s...", poller.Repo.Spec.Url)
	remoteConfig := &config.RemoteConfig{
		Name: "origin",
		URLs: []string{poller.Repo.Spec.Url},
	}
	refs, err := poller.gitRemote.ListReferences(memory.NewStorage(), remoteConfig, &git.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	lastScheduledRef := poller.Repo.Status.GitSHA
	if ok, masterHash := HasNewRevision(refs, lastScheduledRef); ok {
		err := poller.repoStatusManager.SetNewJobRun(poller.Repo, masterHash)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		klog.Infof("No pending commits to run... nothing to do.")
	}
}

func HasNewRevision(refs []*plumbing.Reference, previousHash string) (bool, string) {
	masterRef := FindMasterRef(refs)
	masterHash := masterRef.Hash().String()
	if masterHash != previousHash {
		klog.Infof("Found new commit reference %s... Previous was %s", masterHash, previousHash)
		return true, masterHash
	}
	return false, ""
}

func FindMasterRef(refs []*plumbing.Reference) plumbing.Reference {
	var master plumbing.Reference
	for _, ref := range refs {
		if ref.Name() == plumbing.Master {
			klog.Infof("Repo master HEAD is %s", ref.Hash().String())
			master = *ref
			break
		}
	}
	return master
}
