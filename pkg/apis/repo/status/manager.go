package status

import (
	"fmt"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"

	repo "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	clientset "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned"
)

// Allows changing a Repo resource state.
// Objects passed by the informer must not be modified.
// Hence the DeepCopy() before every operation.
type RepoStatusManager struct {
	repoclientset clientset.Interface
}

func NewRepoStatusManager(repoclientset clientset.Interface) RepoStatusManager {
	return RepoStatusManager{
		repoclientset: repoclientset,
	}
}

func (statusManager RepoStatusManager) update(repo *repo.Repo) error {
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Repo resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := statusManager.repoclientset.RepoV1alpha1().Repos(repo.Namespace).Update(repo)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return errors.Wrapf(err, "updating object %v/%v failed", repo.Namespace, repo.Name)
		}

		// Upsert can be handled by the API server depending on cluster configuration.
		// This is to compensate for those cases and for the lack of it in the client-go testing lib
		// See https://github.com/kubernetes/client-go/issues/479
		_, err = statusManager.repoclientset.RepoV1alpha1().Repos(repo.Namespace).Create(repo)
		if err != nil {
			return errors.Wrapf(err, "creating object %v/%v failed", repo.Namespace, repo.Name)
		}
	}
	return err
}

// Set desired state as new job run
func (statusManager RepoStatusManager) SetNewJobRun(repo *repo.Repo, newGitSha string) error {
	repoCopy := repo.DeepCopy()
	repoCopy.Status.RunJobName = fmt.Sprintf("terraform-run-%s", newGitSha)
	repoCopy.Status.GitSHA = newGitSha
	repoCopy.Status.RunStatus = "New"
	return statusManager.update(repoCopy)
}

func (statusManager RepoStatusManager) SetJobRunStatus(repo *repo.Repo, job *batchv1.Job) error {
	// TODO set last ran datetime & last run status/run output
	repoCopy := repo.DeepCopy()
	repoCopy.Status.RunStatus = determineRunStatus(job)
	return statusManager.update(repo)
}

func (statusManager RepoStatusManager) IsNewRepoRun(repo *repo.Repo) bool {
	return repo.Status.RunStatus == "New"
}

func determineRunStatus(job *batchv1.Job) string {
	if job.Status.Active != 0 {
		return "Running"
	}

	if job.Status.Succeeded != 0 {
		return "Completed"
	} else if job.Status.Failed != 0 {
		return "Failed"
	}

	return "Pending"
}
