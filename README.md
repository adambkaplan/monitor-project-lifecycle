Template Lifecycle Monitoring Application
=========================================
Monitoring application to ensure that OpenShift Origin's templating system is functioning properly.
The app periodically executes a smoketest to ensure that a simple `Template` can be created, and launch a valid `TemplateInstance` from said template. 
Test metrics can be gathered by Prometheus at the `/metrics` endpoint published by the app.

Installation
------------
This procedure assumes that you have developer access to a running OpenShift cluster.

1. Check out this repository from Github: 
```
git clone https://github.com/adambkaplan/openshift-template-monitor.git
```
2. Create an OpenShift project to run the application in - the recommended default is `template-monitor`
```
$: oc new-project template-monitor
```
3. Run the install script:
```
$: cd install
$: hack/deploy
```
4. Log into the OpenShift web console. Open the project created above, and launch the "Template Monitor" application from service catalog.
5. Designate the role name and namespace used by the application to 


Metrics
-------
The following test-specific metrics can be gathered by Prometheus:

1. `template_test_last_ran` [Gauge] - UNIX time in seconds that the test last ran.
2. `template_test_launch_duration_seconds` [Gauge] - the time in seconds that it took for the `TemplateInstance` in the test to be ready.
3. `template_test_total_duration_seconds` [Gauge] - the total time in seconds that the test completed in.

Each metric above can have the following labels:

1. `result` - can be either `success` or `failure`
2. `reason` - if the result is `failure`, provides one of the following reasons that the test failed:

    1. `InitTestFailed`
    2. `CreateTemplateFailed`
    3. `CreateTemplateInstanceFailed`
    4. `LaunchTemplateInstanceFailed`
    5. `LaunchTemplateInstanceTimeout`
    6. `ValidateTemplateInstanceFailed`
    7. `Unknown`

Recommended Prometheus Queries
------------------------------

1. Absolute test failures
```
template_test_last_ran{result="failure", reason=~"InitTestFailed|CreateTemplateFailed|CreateTemplateInstanceFailed|Unknown"}
```
2. Failure to launch/validate template
```
template_test_last_ran{result="failure", reason=~"LaunchTemplateInstanceFailed|LaunchTemplateInstanceTimeout|ValidateTemplateInstanceFailed"}
```
3. Performance of successful template launches
```
template_test_launch_duration_seconds{result="success"}
```
4. Overall successful test performance
```
template_test_total_duration_seconds{result="success"}
```
