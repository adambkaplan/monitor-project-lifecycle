package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adambkaplan/openshift-template-monitor/pkg/templates"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultPort        = ":8080"
	defaultKeepObjects = false
	defaultInterval    = 300
	defaultTimeout     = 60
)

var (
	addr           string
	keepObjects    bool
	interval       int
	timeout        int
	metricsHandler http.Handler
)

func init() {
	flag.StringVar(&addr, "listen-address", defaultPort, "The address to listen on for HTTP requests.")
	flag.BoolVar(&keepObjects, "keep-objects", defaultKeepObjects, "Keep objects created by the smoketest")
	flag.IntVar(&interval, "interval", defaultInterval, "Interval to run the smoketest job (seconds)")
	flag.IntVar(&timeout, "timeout", defaultTimeout, "Timeout for launching a Template Instance (seconds)")
	flag.Parse()
}

func main() {
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)

	glog.V(0).Info("Started template smoketest application")
	glog.V(2).Infof("Listening at address %s", addr)
	glog.V(2).Infof("Keeping test artifact objects: %t", keepObjects)
	glog.V(2).Infof("Test interval: %d", interval)
	glog.V(2).Infof("Instance launch timeout: %d", timeout)

	metricsHandler = prometheus.Handler()
	http.HandleFunc("/healthz", handleHealthz)
	http.HandleFunc("/metrics", handleMetrics)

	templateTestGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "template_test_last_ran",
			Help: "Time that the template smoketest last ran",
		},
		[]string{"result", "reason"},
	)
	templateLaunchGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "template_test_launch_duration_seconds",
			Help: "Duration the cluster last took to launch a test template instance.",
		},
		[]string{"result", "reason"},
	)
	totalDurationGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "template_test_total_duration_seconds",
			Help: "Total duration of the previous test.",
		},
		[]string{"result", "reason"},
	)
	prometheus.MustRegister(templateTestGauge, templateLaunchGauge, totalDurationGauge)

	go http.ListenAndServe(addr, nil)
	go runTemplateSmoketest(time.Duration(interval)*time.Second, templateTestGauge, templateLaunchGauge, totalDurationGauge)

	<-exit
	glog.V(0).Info("Exiting template smoketest application")
	glog.Flush()
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok")
	glog.V(1).Info("GET /healthz")
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	metricsHandler.ServeHTTP(w, r)
	glog.V(1).Info("GET /metrics")
}

func runTemplateSmoketest(interval time.Duration, testGauge, launchGauge, durationGauge *prometheus.GaugeVec) {
	glog.V(0).Info("Running template controller smoketests")
	first := true
	for {
		if !first {
			time.Sleep(interval)
		} else {
			first = false
		}
		doSmoketest(testGauge, launchGauge, durationGauge)
	}
}

func doSmoketest(testGauge, launchGauge, durationGauge *prometheus.GaugeVec) {
	var launchDuration, totalDuration float64
	start := time.Now()
	test, err := templates.NewSmoketest()
	if err != nil {
		totalDuration = time.Now().Sub(start).Seconds()
		glog.Errorf("Failed initiating smoketest: %s", err)
		publishResult(testGauge, launchGauge, durationGauge, launchDuration, totalDuration, err)
		return
	}
	launchDuration, err = test.Run(keepObjects, timeout)
	totalDuration = time.Now().Sub(start).Seconds()
	publishResult(testGauge, launchGauge, durationGauge, launchDuration, totalDuration, err)
}

func publishResult(testGauge, launchGauge, durationGauge *prometheus.GaugeVec, launchDuration, totalDuration float64, err error) {
	result := "success"
	var reason string
	if err != nil {
		result = "failure"
		reason = err.Error()
	}
	testGauge.WithLabelValues(result, reason).SetToCurrentTime()
	launchGauge.WithLabelValues(result, reason).Set(launchDuration)
	durationGauge.WithLabelValues(result, reason).Set(totalDuration)
}
