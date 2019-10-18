package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	batchclientset "k8s.io/client-go/kubernetes/typed/batch/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	status "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/status"
	clientset "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned"
	informers "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/informers/externalversions"
	"github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	batchClient, err := batchclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building batch clientset: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	repoClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	repoInformerFactory := informers.NewSharedInformerFactory(repoClient, time.Second*30)

	repoStatusManager := status.NewRepoStatusManager(repoClient)

	controller := NewController(
		batchClient,
		kubeClient,
		repoStatusManager,
		kubeInformerFactory.Batch().V1().Jobs(),
		repoInformerFactory.Repo().V1alpha1().Repos())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	repoInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
