package main

import (
	"reflect"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	status "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/status"
	repov1alpha1 "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/apis/repo/v1alpha1"
	"github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/clientset/versioned/fake"
	informers "github.com/davidmontoyago/di-terraform-repo-pull-controller/pkg/generated/informers/externalversions"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	repoclient  *fake.Clientset
	batchclient *k8sfake.Clientset
	kubeclient  *k8sfake.Clientset
	// Objects to put in the store.
	reposLister []*repov1alpha1.Repo
	jobsLister  []*batchv1.Job
	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
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

func (f *fixture) newController() (*Controller, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.batchclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)
	f.repoclient = fake.NewSimpleClientset(f.objects...)
	repoStatusManager := status.NewRepoStatusManager(f.repoclient)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())
	repoInformerFactory := informers.NewSharedInformerFactory(f.repoclient, noResyncPeriodFunc())

	c := NewController(f.batchclient.BatchV1(), f.kubeclient, repoStatusManager,
		kubeInformerFactory.Batch().V1().Jobs(), repoInformerFactory.Repo().V1alpha1().Repos())

	c.reposSynced = alwaysReady
	c.jobsSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, f := range f.reposLister {
		repoInformerFactory.Repo().V1alpha1().Repos().Informer().GetIndexer().Add(f)
	}

	for _, d := range f.jobsLister {
		kubeInformerFactory.Batch().V1().Jobs().Informer().GetIndexer().Add(d)
	}

	return c, repoInformerFactory, kubeInformerFactory
}

func (f *fixture) run(repoName string) {
	f.runController(repoName, true, false)
}

func (f *fixture) runExpectError(repoName string) {
	f.runController(repoName, true, true)
}

func (f *fixture) runController(repoName string, startInformers bool, expectError bool) {
	c, repoInformerFactory, kubeInformerFactory := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		repoInformerFactory.Start(stopCh)
		kubeInformerFactory.Start(stopCh)
	}

	err := c.syncHandler(repoName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing repo: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing repo, got nil")
	}

	actions := filterInformerActions(f.repoclient.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	k8sActions := filterInformerActions(f.batchclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "repos") ||
				action.Matches("watch", "repos") ||
				action.Matches("list", "jobs") ||
				action.Matches("watch", "jobs")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectCreateJobAction(d *batchv1.Job) {
	f.kubeactions = append(f.kubeactions, core.NewCreateAction(schema.GroupVersionResource{Resource: "jobs"}, d.Namespace, d))
}

func (f *fixture) expectUpdateDeploymentAction(d *batchv1.Job) {
	f.kubeactions = append(f.kubeactions, core.NewUpdateAction(schema.GroupVersionResource{Resource: "jobs"}, d.Namespace, d))
}

func (f *fixture) expectUpdateRepoStatusAction(repo *repov1alpha1.Repo) {
	updateAction := core.NewUpdateAction(schema.GroupVersionResource{Resource: "repos"}, repo.Namespace, repo)
	// TODO: Until #38113 is merged, we can't use Subresource
	//action.Subresource = "status"
	f.actions = append(f.actions, updateAction)

	createAction := core.NewCreateAction(schema.GroupVersionResource{Resource: "repos", Version: "v1alpha", Group: "repo.terraform.gitops.k8s.io"}, repo.Namespace, repo)
	f.actions = append(f.actions, createAction)
}

func getKey(repo *repov1alpha1.Repo, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(repo)
	if err != nil {
		t.Errorf("Unexpected error getting key for repo %v: %v", repo.Name, err)
		return ""
	}
	return key
}

func TestCreatesJob(t *testing.T) {
	f := newFixture(t)
	repo := newRepo("test-repo")
	repo.Status.RunStatus = "New"

	f.reposLister = append(f.reposLister, repo)
	f.objects = append(f.objects, repo)

	expJob := newJob(repo)
	f.expectCreateJobAction(expJob)
	f.expectUpdateRepoStatusAction(repo)

	f.run(getKey(repo, t))
}

func TestDoNothingWhenResourceHasNoJobToRun(t *testing.T) {
	f := newFixture(t)
	repo := newRepo("test-repo")
	job := newJob(repo)

	f.reposLister = append(f.reposLister, repo)
	f.objects = append(f.objects, repo)
	f.jobsLister = append(f.jobsLister, job)
	f.kubeobjects = append(f.kubeobjects, job)

	f.expectUpdateRepoStatusAction(repo)
	f.run(getKey(repo, t))
}

func TestNotControlledByResource(t *testing.T) {
	f := newFixture(t)
	repo := newRepo("test")
	job := newJob(repo)

	job.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

	f.reposLister = append(f.reposLister, repo)
	f.objects = append(f.objects, repo)
	f.jobsLister = append(f.jobsLister, job)
	f.kubeobjects = append(f.kubeobjects, job)

	f.runExpectError(getKey(repo, t))
}

func int32Ptr(i int32) *int32 { return &i }
