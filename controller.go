package main

import (
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	batchclientset "k8s.io/client-go/kubernetes/typed/batch/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	repov1alpha1 "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	clientset "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned"
	samplescheme "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/informers/externalversions/repo/v1alpha1"
	listers "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/listers/repo/v1alpha1"
	poller "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/poller"
)

const controllerAgentName = "repo-gitops-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Repo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Repo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Repo"
	// MessageResourceSynced is the message used for an Event fired when a Repo
	// is synced successfully
	MessageResourceSynced = "Repo synced successfully"
)

// Controller is the controller implementation for Repo resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	batchclientset batchclientset.BatchV1Interface
	// repoclientset is a clientset for our own API group
	repoclientset clientset.Interface

	jobsLister  batchlisters.JobLister
	jobsSynced  cache.InformerSynced
	reposLister listers.RepoLister
	reposSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// keeps references to polling goroutines by repo key
	repoPollers map[string]*poller.RepoPoller
}

func NewController(
	batchclientset batchclientset.BatchV1Interface,
	kubeclientset kubernetes.Interface,
	repoclientset clientset.Interface,
	jobInformer batchinformers.JobInformer,
	repoInformer informers.RepoInformer) *Controller {

	// Create event broadcaster
	// Add repo-controller types to the default Kubernetes Scheme so Events can be
	// logged for repo-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		batchclientset: batchclientset,
		repoclientset:  repoclientset,
		jobsLister:     jobInformer.Lister(),
		jobsSynced:     jobInformer.Informer().HasSynced,
		reposLister:    repoInformer.Lister(),
		reposSynced:    repoInformer.Informer().HasSynced,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Repos"),
		recorder:       recorder,
		repoPollers:    make(map[string]*poller.RepoPoller),
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Repo resources change
	// TODO terminate poller on Repo deletion
	repoInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueRepo,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueRepo(new)
		},
	})
	// Set up an event handler for when Job resources change. This
	// handler will lookup the owner of the given Job, and if it is
	// owned by a Job resource will enqueue that Job resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Job resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleJob,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*batchv1.Job)
			oldDepl := old.(*batchv1.Job)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleJob(new)
		},
		DeleteFunc: controller.handleJob,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Repo controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.jobsSynced, c.reposSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Repo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Repo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Repo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Repo resource with this namespace/name
	repo, err := c.reposLister.Repos(namespace).Get(name)
	if err != nil {
		// The Repo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("repo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	jobName := repo.Status.RunJobName
	runStatus := repo.Status.RunStatus
	if jobName == "" || runStatus != "New" {
		// Repo has not a Job to run or it has already run. Nothing to do
		klog.Infof("Repo has no pending Job to run [last known run status: %s].", runStatus)
		return nil
	}

	// Get the job with the last job name specified in Repo.Status
	job, err := c.jobsLister.Jobs(repo.Namespace).Get(jobName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		job, err = c.batchclientset.Jobs(repo.Namespace).Create(newJob(repo))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Job is not controlled by this Repo resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(job, repo) {
		msg := fmt.Sprintf(MessageResourceExists, job.Name)
		c.recorder.Event(repo, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the Repo resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	// if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
	// 	klog.V(4).Infof("Foo %s replicas: %d, deployment replicas: %d", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)
	// 	deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Update(newJob(foo))
	// }

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Repo resource to reflect the
	// current state of the world
	err = c.updateRepoStatus(repo, job)
	if err != nil {
		return err
	}

	c.recorder.Event(repo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateRepoStatus(repo *repov1alpha1.Repo, job *batchv1.Job) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	repoCopy := repo.DeepCopy()

	// currentTime := time.Now()
	// TODO set last ran datetime & last run status/run output

	repoCopy.Status.RunStatus = DetermineRunStatus(job)

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Repo resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.repoclientset.RepoV1alpha1().Repos(repo.Namespace).Update(repoCopy)
	return err
}

func DetermineRunStatus(job *batchv1.Job) string {
	var runStatus string
	if job.Status.Active == 0 {
		if job.Status.Succeeded != 0 {
			runStatus = "Completed"
		} else if job.Status.Failed != 0 {
			runStatus = "Failed"
		} else {
			runStatus = "Pending"
		}
	} else {
		runStatus = "Running"
	}
	return runStatus
}

// enqueueRepo takes a Repo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Repo.
func (c *Controller) enqueueRepo(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	if _, found := c.repoPollers[key]; !found {
		klog.Infof("Starting repo poller for '%s'...", key)
		repo := obj.(*repov1alpha1.Repo)
		var repoCopy repov1alpha1.Repo = *repo.DeepCopy()
		repoPoller := poller.NewRepoPoller(key, repoCopy, c.repoclientset)
		repoPoller.Start()
		c.repoPollers[key] = repoPoller
	}

	c.workqueue.Add(key)
}

// handleJob will take any resource implementing metav1.Object and attempt
// to find the Repo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Repo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleJob(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Foo, we should not do anything more
		// with it.
		if ownerRef.Kind != "Repo" {
			return
		}

		repo, err := c.reposLister.Repos(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of repo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		job := obj.(*batchv1.Job)
		// TODO clone repo
		repo.Status.RunStatus = DetermineRunStatus(job)

		c.enqueueRepo(repo)
		return
	}
}

// newJob creates a new Job for a Repo resource. It also sets
// the appropriate OwnerReferences on the resource so handleJob can discover
// the Repo resource that 'owns' it.
func newJob(repo *repov1alpha1.Repo) *batchv1.Job {
	labels := map[string]string{
		"app":        repo.Name,
		"controller": "repos.terraform.gitops.k8s.io",
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repo.Status.RunJobName,
			Namespace: repo.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(repo, repov1alpha1.SchemeGroupVersion.WithKind("Repo")),
			},
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "terraform-run",
							Image:           "terraform-runner:latest",
							ImagePullPolicy: corev1.PullNever,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}
