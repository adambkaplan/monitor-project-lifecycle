package templates

import (
	"testing"

	templateV1 "github.com/openshift/api/template/v1"
	fakeV1 "github.com/openshift/client-go/template/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// TODO: Add unit tests for the following:
// NewSmoketest
// launchTemplateInstanceCheck
// validateTemplateInstance
// deleteSecret
// deleteTemplateInstance
