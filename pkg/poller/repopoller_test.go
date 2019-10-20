package poller

import (
	"testing"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/status"
	repov1alpha1 "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	"github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned/fake"
)

type GitRemoteTest struct {
}

func (d GitRemoteTest) ListReferences(
	s storage.Storer,
	c *config.RemoteConfig,
	o *git.ListOptions,
) (rfs []*plumbing.Reference, err error) {
	return []*plumbing.Reference{
		plumbing.NewReferenceFromStrings("refs/heads/master", "f7b877701fbf855b44c0a9e86f3fdce2c298b07f"),
	}, nil
}

func TestUpdateRepoStatusWithGitCommitSHA(t *testing.T) {
	repo := newRepo("test-repo")
	repoclient := fake.NewSimpleClientset(repo)
	repoStatusManager := status.NewRepoStatusManager(repoclient)

	poller := NewRepoPoller("default/example-repo", repo, repoStatusManager, GitRemoteTest{})
	poller.CheckForNewRevisions()

	expectedGitSHA := "f7b877701fbf855b44c0a9e86f3fdce2c298b07f"
	if poller.Repo.Status.GitSHA != expectedGitSHA {
		t.Errorf("got = %s; want %s", poller.Repo.Status.GitSHA, expectedGitSHA)
	}
}

func newRepo(name string) *repov1alpha1.Repo {
	return &repov1alpha1.Repo{
		TypeMeta: metav1.TypeMeta{APIVersion: repov1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: repov1alpha1.RepoSpec{
			Url: "https://github.com/davidmontoyago/some-repo.git",
		},
	}
}
