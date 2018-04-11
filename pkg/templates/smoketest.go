package templates

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/golang/glog"
	templatev1 "github.com/openshift/api/template/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const configMapJSON = `
{
	"apiVersion": "v1",
	"kind": "ConfigMap",
	"metadata": {
		"name": "test-configmap-${ID}"
	},
	"data": {
		"foo": "bar",
		"simpleParam": "${SIMPLE_PARAM}"
	}
}
`

// centos:7 should be cached in an OpenShift cluster
const jobJSON = `
{
	"apiVersion":  "batch/v1",
	"kind": "Job",
	"metadata": {
		"name": "test-job-${ID}"
	},
	"spec": {
		"backoffLimit": 1,
		"template": {
			"spec": {
				"restartPolicy": "Never",
				"containers": [
					{
						"name": "bash",
						"image": "centos:7",
						"command": [ "/bin/bash", "-c", "--" ],
						"args": "${{JSON_PARAM}}"
					}
				]
			}
		}
	}
}
`

var (
	// ErrInitTest is returned if the Smoketest could not be initialized.
	ErrInitTest = errors.New("InitTestFailed")
	// ErrCreateTemplate is returned if the Smoketest could not create the `Template`.
	ErrCreateTemplate = errors.New("CreateTemplateFailed")
	// ErrCreateInstance is returend if the Smoketest could not create the `TemplateInstance`.
	ErrCreateInstance = errors.New("CreateTemplateInstanceFailed")
	// ErrLaunchInstanceFailed is returned if the Smoketest could not launch the `TemplateInstance`,
	// or any of the components in the template instance could not be accessed.
	ErrLaunchInstanceFailed = errors.New("LaunchTemplateInstanceFailed")
	// ErrLaunchInstanceTimeout is returned if the Smoketest timed out waiting for the `TemplateInstance` to launch.
	ErrLaunchInstanceTimeout = errors.New("LaunchTemplateInstanceTimeout")
	// ErrInstanceInvalid is returned if the `TemplateInstance` was not configured properly.
	ErrInstanceInvalid = errors.New("ValidateTemplateInstanceFailed")
	// ErrUnknown is returned if an error unrelated to the test is found.
	ErrUnknown = errors.New("Unknown")
)

// Smoketest runs sanity checks against the OpenShift Template and TemplateInstance controllers.
type Smoketest struct {
	namespace         string
	templateInterface templatev1client.TemplateV1Interface
	projectInterface  projectv1client.ProjectV1Interface
	k8sInterface      kubernetes.Interface
}

// NewSmoketest creates a new `Smoketest` instance to run sanity checks and configures the OpenShift API client.
func NewSmoketest() (*Smoketest, error) {
	smoketest := &Smoketest{}
	err := smoketest.init()
	return smoketest, err
}

func (t *Smoketest) init() error {
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	namespace, _, err := kubeconfig.Namespace()
	if err != nil {
		glog.Warningf("Failed to load kubernetes namespace: %v", err)
		return ErrInitTest
	}
	t.namespace = namespace
	// Get a rest.Config from the kubeconfig file.  This will be passed into all
	// the client objects we create.
	restconfig, err := kubeconfig.ClientConfig()
	if err != nil {
		glog.Warningf("Failed to create kubernetes REST client config: %v", err)
		return ErrInitTest
	}
	tClient, err := templatev1client.NewForConfig(restconfig)
	if err != nil {
		glog.Warningf("Failed to create kubernetes REST client for templates: %v", err)
		return ErrInitTest
	}
	pClient, err := projectv1client.NewForConfig(restconfig)
	if err != nil {
		glog.Warningf("Failed to create kubernetes REST client for projects: %v", err)
		return ErrInitTest
	}
	k8sClient, err := kubernetes.NewForConfig(restconfig)
	if err != nil {
		glog.Warningf("failed to create kubernetes REST client for core: %v", err)
		return ErrInitTest
	}
	t.templateInterface = tClient
	t.projectInterface = pClient
	t.k8sInterface = k8sClient
	return nil
}

// Run executes the smoketest for the `Template` and `TemplateInstance` controllers.
// This will perform the following actions within the current namespace:
//
// 1. Create a parameterized `Template` with the following:
//   a. a `ConfigMap` with simple key-value pairs
//   b. a batch `Job` that executes a bash command
// 2. Launch a `TemplateInstance` from the above template, with simple parameters configured via a `Secret`
func (t *Smoketest) Run(keepObjects bool, timeout int) (float64, error) {
	workspace := t.namespace
	id := strconv.FormatInt(time.Now().Unix(), 10)
	glog.V(1).Infof("Started running template smoketest %s", id)
	defer glog.V(1).Infof("Completed template smoketest %s", id)
	template, err := t.createTemplateCheck(workspace, id)
	if !keepObjects {
		defer t.deleteTemplate(workspace, template)
	}
	if err != nil {
		glog.Warningf("Failed testing template: %v", err)
		return 0, err
	}
	ti, secret, duration, err := t.launchTemplateInstanceCheck(workspace, template.Name, id, timeout)
	if !keepObjects {
		defer t.deleteSecret(workspace, secret)
	}
	if !keepObjects {
		defer t.deleteTemplateInstance(workspace, ti)
	}
	if err != nil {
		glog.Warningf("Failed testing template instance: %v", err)
		return duration, err
	}
	glog.V(1).Infof("Successfully ran template smoketest %s", id)
	return duration, nil
}

// createTemplateCheck runs a smoke test to ensure that a `Template` can be created.
func (t *Smoketest) createTemplateCheck(namespace string, id string) (*templatev1.Template, error) {
	glog.V(1).Info("Checking that a template can be created")
	defer glog.V(1).Info("Completed template creation check")
	templateName := fmt.Sprintf("smoketest-template-%s", id)
	var testTemplate = &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name: templateName,
		},
		Objects: []runtime.RawExtension{
			runtime.RawExtension{
				Raw: []byte(configMapJSON),
			},
			runtime.RawExtension{
				Raw: []byte(jobJSON),
			},
		},
		ObjectLabels: map[string]string{
			"this":   "that",
			"google": "kubernetes",
			"redhat": "openshift",
		},
		Parameters: []templatev1.Parameter{
			templatev1.Parameter{
				Name:        "ID",
				Description: "An identifier for all objects in the template instance.",
				DisplayName: "ID",
			},
			templatev1.Parameter{
				Name:        "SIMPLE_PARAM",
				Description: "A simple parameter for a template.",
				DisplayName: "Simple Parameter",
			},
			templatev1.Parameter{
				Name:        "JSON_PARAM",
				Description: "A JSON or YAML-formatted parameter.",
				DisplayName: "JSON Parameter",
			},
		},
	}
	result, err := t.templateInterface.Templates(namespace).Create(testTemplate)
	if err != nil {
		glog.Warningf("Failed to create template: %v", err)
		return result, ErrCreateTemplate
	}
	glog.V(2).Infof("Created template %s", result.Name)
	return result, nil
}

// deleteSecret deletes a `Secret` in the given namespace.
func (t *Smoketest) deleteSecret(namespace string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	err := t.k8sInterface.CoreV1().Secrets(namespace).Delete(secret.Name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Warningf("Failed to delete secret %s: %v", secret.Name, err)
		return err
	}
	glog.V(2).Infof("Deleted secret %s", secret.Name)
	return nil
}

// deleteTemplate deletes a template in the given namespace.
func (t *Smoketest) deleteTemplate(namespace string, template *templatev1.Template) error {
	if template == nil {
		return nil
	}
	err := t.templateInterface.Templates(namespace).Delete(template.Name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Warningf("Failed to delete template %s: %v", template.Name, err)
		return err
	}
	glog.V(2).Infof("Deleted template %s", template.Name)
	return nil
}

// deleteTemplateInstance deletes a template instance in the given namespace.
// Deletion cascades to all child objects in the `TemplateInstance`, and occurs in the foreground.
func (t *Smoketest) deleteTemplateInstance(namespace string, instance *templatev1.TemplateInstance) error {
	if instance == nil {
		return nil
	}
	deletePolicy := metav1.DeletePropagationForeground
	err := t.templateInterface.TemplateInstances(namespace).Delete(instance.Name, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		glog.Warningf("Failed to delete template instance %s: %s", instance.Name, err)
		return err
	}
	glog.V(2).Infof("Deleted template instance %s\n", instance.Name)
	return nil
}

// getDummyTemplateParams returns dummy template parameters to use when creating a TemplateInstance from a Template.
func (t *Smoketest) getDummyTemplateParams(id string) map[string]string {
	return map[string]string{
		"ID":           id,
		"SIMPLE_PARAM": "test",
		"JSON_PARAM":   "[ \"echo\", \"Hello world\" ]",
	}
}

// launchTemplateInstanceCheck runs a smoke test to ensure a TemplateInstance can be launched from a Template.
func (t *Smoketest) launchTemplateInstanceCheck(namespace string, templateName string, id string, timeoutInterval int) (*templatev1.TemplateInstance, *corev1.Secret, float64, error) {
	glog.V(1).Info("Checking that an instance can be launched from a template")
	defer glog.V(1).Info("Completed template instance launch check")
	var duration float64
	params := t.getDummyTemplateParams(id)
	template, err := t.templateInterface.Templates(namespace).Get(templateName, metav1.GetOptions{})
	if err != nil {
		glog.Warningf("Failed to get template %s details: %v", templateName, err)
		return nil, nil, duration, ErrCreateTemplate
	}
	glog.V(2).Infof("Fetched template %s\n", template.Name)
	data := make(map[string][]byte)
	for k, v := range params {
		data[k] = []byte(v)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-secret", templateName),
		},
		Data: data,
	}
	secretResult, err := t.k8sInterface.CoreV1().Secrets(namespace).Create(secret)
	if err != nil {
		glog.Warningf("Failed to create secret %s for template instance: %v", secret.Name, err)
		return nil, secretResult, duration, ErrCreateInstance
	}
	glog.V(2).Infof("Created secret %s", secretResult.Name)
	launchStart := time.Now()
	ti, err := t.templateInterface.TemplateInstances(namespace).Create(&templatev1.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-instance", templateName),
		},
		Spec: templatev1.TemplateInstanceSpec{
			Template: *template,
			Secret: &corev1.LocalObjectReference{
				Name: secret.Name,
			},
		},
	})
	if err != nil {
		glog.Warningf("Failed to create template instance: %v", err)
		return nil, secretResult, duration, ErrCreateInstance
	}
	glog.V(2).Infof("Created template instance %s", ti.Name)
	watcher, err := t.templateInterface.TemplateInstances(namespace).Watch(
		metav1.SingleObject(ti.ObjectMeta),
	)
	if err != nil {
		glog.Warningf("Failed to watch template instance %s: %v", ti.Name, err)
		return ti, secretResult, duration, ErrUnknown
	}
	glog.V(2).Infof("Waiting for template instance %s to be ready...", ti.Name)
	timeout := time.After(time.Duration(timeoutInterval) * time.Second)
	for {
		select {
		case <-timeout:
			duration = time.Now().Sub(launchStart).Seconds()
			return ti, secretResult, duration, ErrLaunchInstanceTimeout
		case event := <-watcher.ResultChan():
			switch event.Type {
			case watch.Modified:
				duration = time.Now().Sub(launchStart).Seconds()
				ti = event.Object.(*templatev1.TemplateInstance)

				for _, cond := range ti.Status.Conditions {
					// If the TemplateInstance contains a status condition
					// Ready == True, stop watching.
					if cond.Type == templatev1.TemplateInstanceReady &&
						cond.Status == corev1.ConditionTrue {
						watcher.Stop()
						glog.V(2).Infof("Template instance %s is ready", ti.Name)
						err = t.validateTemplateInstance(ti, template, params)
						return ti, secretResult, duration, err
					}

					// If the TemplateInstance contains a status condition
					// InstantiateFailure == True, indicate failure.
					if cond.Type ==
						templatev1.TemplateInstanceInstantiateFailure &&
						cond.Status == corev1.ConditionTrue {
						watcher.Stop()
						glog.Warningf("Failed to instantiate template instance %s", ti.Name)
						return ti, secretResult, duration, ErrLaunchInstanceFailed
					}
				}

			default:
				duration = time.Now().Sub(launchStart).Seconds()
				glog.Errorf("Unexpected event type %s watching template instance %s", event.Type, ti.Name)
				return ti, secretResult, duration, ErrUnknown
			}
		}
	}
}

func (t *Smoketest) validateTemplateInstance(instance *templatev1.TemplateInstance, template *templatev1.Template, params map[string]string) error {
	if !reflect.DeepEqual(template.Labels, instance.Labels) {
		glog.Warningf("Labels for template %s {%s} and instance %s {%s} do not match", template.Name, template.Labels, instance.Name, instance.Labels)
		return ErrInstanceInvalid
	}
	configMapName := fmt.Sprintf("test-configmap-%s", params["ID"])
	configMap, err := t.k8sInterface.CoreV1().ConfigMaps(t.namespace).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		return ErrLaunchInstanceFailed
	}
	expectedData := map[string]string{
		"foo":         "bar",
		"simpleParam": params["SIMPLE_PARAM"],
	}
	if !reflect.DeepEqual(expectedData, configMap.Data) {
		glog.Warningf("Data in config map %s does not match expected value %s", configMap.Data, expectedData)
		return ErrInstanceInvalid
	}
	expectedArgs := make([]string, 0)
	err = json.Unmarshal([]byte(params["JSON_PARAM"]), &expectedArgs)
	if err != nil {
		glog.Errorf("Could not decode expected JSON: %v", err)
		return ErrUnknown
	}
	jobName := fmt.Sprintf("test-job-%s", params["ID"])
	job, err := t.k8sInterface.BatchV1().Jobs(t.namespace).Get(jobName, metav1.GetOptions{})
	if err != nil {
		glog.Warningf("Could not fetch details of job %s: %v", jobName, err)
		return ErrLaunchInstanceFailed
	}
	actualArgs := job.Spec.Template.Spec.Containers[0].Args
	if !reflect.DeepEqual(expectedArgs, actualArgs) {
		glog.Warningf("Arguments for instance job %s do not match expected value %s", actualArgs, expectedArgs)
		return ErrInstanceInvalid
	}
	glog.V(2).Infof("Validated template instance %s correctly launched from template %s", instance.Name, template.Name)
	return nil
}
