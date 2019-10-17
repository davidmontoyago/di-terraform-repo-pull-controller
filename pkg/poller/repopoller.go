package poller

import (
	"fmt"
	"log"
	"time"

	repo "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	clientset "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"k8s.io/klog"
)

const POLLING_FREQUENCY_SECONDS = 30

type RepoPoller struct {
	RepoKey       string
	Repo          repo.Repo
	Ticker        *time.Ticker
	Done          chan bool
	repoclientset clientset.Interface
}

func NewRepoPoller(repoKey string,
	repo repo.Repo,
	repoclientset clientset.Interface) *RepoPoller {

	ticker := time.NewTicker(POLLING_FREQUENCY_SECONDS * time.Second)
	done := make(chan bool)

	return &RepoPoller{
		RepoKey:       repoKey,
		Repo:          repo,
		Ticker:        ticker,
		Done:          done,
		repoclientset: repoclientset,
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
	lastScheduledRef := poller.Repo.Status.GitSHA
	if masterHash != lastScheduledRef {
		klog.Infof("Found new commit reference %s... Previous was %s", masterHash, lastScheduledRef)
		repo := &poller.Repo
		repo.Status.RunJobName = fmt.Sprintf("terraform-run-%s", masterHash)
		repo.Status.GitSHA = masterHash
		repo.Status.RunStatus = "New"
		_, err := poller.repoclientset.RepoV1alpha1().Repos(repo.Namespace).Update(repo)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		klog.Infof("No pending commits to run... nothing to do.")
	}
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
