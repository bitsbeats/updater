package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	// Config holds the configuration specified via environment
	Config struct {
		Namespace  string `envconfig:"NAMESPACE" required:"true"`
		Deployment string `envconfig:"DEPLOYMENT" required:"true"`
		Listen     string `envconfig:"LISTEN" default:":8080"`
		Token      []byte `envconfig:"TOKEN" reqired:"true"`

		LinuxHome   string `envconfig:"HOME"`
		WindowsHome string `envconfig:"USERPROFILE"`
	}
)

func main() {
	// config
	cfg := &Config{}
	err := envconfig.Process("", cfg)
	if err != nil {
		log.Fatalf("unable to parse env: %s", err)
	}
	home := cfg.LinuxHome
	if home == "" {
		home = cfg.WindowsHome
	}
	kubeconfig := filepath.Join(home, ".kube", "config")

	// create kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("unable to load in-cluster config: %s", err)
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("unable to create client: %s", err)
	}

	// add non-app handlers
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok\n")
	}))

	// create app and inject into http
	app := NewApp(cfg, clientset)
	http.HandleFunc("/", app.Handle)
	logged := app.Middleware(http.DefaultServeMux)

	// run webserber
	log.Printf("listening on: %s", cfg.Listen)
	err = http.ListenAndServe(cfg.Listen, logged)
	if err != nil {
		log.Fatalf("unable to listen: %s", err)
	}

}
