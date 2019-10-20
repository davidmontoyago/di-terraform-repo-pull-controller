package poller

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage"
)

type GitRemote interface {
	ListReferences(
		s storage.Storer,
		c *config.RemoteConfig,
		o *git.ListOptions,
	) (rfs []*plumbing.Reference, err error)
}

type GitRemoteDelegator struct {
}

func (d GitRemoteDelegator) ListReferences(
	s storage.Storer,
	c *config.RemoteConfig,
	o *git.ListOptions,
) (rfs []*plumbing.Reference, err error) {
	rem := git.NewRemote(s, c)
	return rem.List(&git.ListOptions{})
}
