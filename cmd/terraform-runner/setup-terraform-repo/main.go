package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"k8s.io/klog"
)

var ()

func main() {
	klog.Infof("Starting terraform-runner...")
	repoName := os.Getenv("REPO_NAME")
	klog.Infof("REPO_NAME=%s", repoName)

	repoUrl := os.Getenv("REPO_URL")
	klog.Infof("REPO_URL=%s", repoUrl)

	gitSHA := os.Getenv("GIT_SHA")
	klog.Infof("GIT_SHA=%s", gitSHA)

	GitCheckout(repoUrl, gitSHA)
	ListRepoContents()
}

func GitCheckout(repoUrl string, gitSHA string) {
	repo, err := git.PlainClone("/workspace", false, &git.CloneOptions{
		URL: repoUrl,
	})
	TerminateIfError(err, "Failed to clone repo: %v")
	klog.Infof("Completed cloning repo %s.", repoUrl)

	worktree, err := repo.Worktree()
	TerminateIfError(err, "Failed to fetch repo work tree: %v")

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(gitSHA),
	})
	TerminateIfError(err, "Failed to checkout revision: %v")
	klog.Infof("Completed repo checkout to %s.", gitSHA)
}

func ListRepoContents() {
	cmd := exec.Command("ls", "-al", "/workspace")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	klog.Infof("Listing repo contents... \n%s", out.String())
}

func TerminateIfError(err error, format string) {
	if err != nil {
		klog.Errorf(format, err)
		os.Exit(1)
	}
}
