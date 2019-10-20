package main

import (
	"os"

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

	// TODO clone repo & plan/apply
}
