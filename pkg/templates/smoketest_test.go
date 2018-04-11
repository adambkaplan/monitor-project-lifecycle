package templates

import (
	"testing"

	templateV1 "github.com/openshift/api/template/v1"
	fakeV1 "github.com/openshift/client-go/template/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCreateTemplateSmoketest(t *testing.T) {
	fakeClient := fakeV1.NewSimpleClientset().Template()
	smoketest := &Smoketest{
		templateInterface: fakeClient,
	}
	_, err := smoketest.createTemplateCheck("dummyNamespace", "testTemplate")
	if err != nil {
		t.Errorf("Create template check failed: %s", err)
	}
}

func TestDeleteTemplateSmoketest(t *testing.T) {
	// Need a fake REST client to spoof responses
	testTemplate := &templateV1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "dummyNamespace",
		},
	}
	fakeClient := fakeV1.NewSimpleClientset(testTemplate).Template()
	smoketest := &Smoketest{
		templateInterface: fakeClient,
	}
	err := smoketest.deleteTemplate("dummyNamespace", testTemplate)
	if err != nil {
		t.Errorf("Delete template check failed: %s", err)
	}
}

// TODO: Figure out how use a fake watcher

func TestLaunchTemplateInstanceSmoketest(t *testing.T) {
	testTemplate := &templateV1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "dummyNamespace",
		},
	}
	id := "12345"
	watchInstance := &templateV1.TemplateInstance{
		Status: templateV1.TemplateInstanceStatus{
			Conditions: []templateV1.TemplateInstanceCondition{
				templateV1.TemplateInstanceCondition{
					Type:   templateV1.TemplateInstanceReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	t.Logf("Set up fixtures")
	fakeClient := fakeV1.NewSimpleClientset(testTemplate)
	fakeWatch := watch.NewFake()
	t.Logf("Set up fake watch")
	go func() {
		fakeWatch.Modify(watchInstance)
		t.Logf("Sent modify event")
	}()
	fakeClient.AddWatchReactor("*", k8stesting.DefaultWatchReactor(fakeWatch, nil))
	t.Logf("Running check")
	k8s := k8sfake.NewSimpleClientset()
	smoketest := &Smoketest{
		templateInterface: fakeClient.Template(),
		k8sInterface:      k8s,
	}
	_, _, err := smoketest.launchTemplateInstanceCheck("dummyNamespace", testTemplate.Name, id)
	if err != nil {
		t.Errorf("Launch template instance check failed: %s", err)
	}
}
