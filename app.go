package main

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type (
	App struct {
		Config    *Config
		Clientset *kubernetes.Clientset
		Counter   *prometheus.CounterVec
	}

	AppResponseWriter struct {
		http.ResponseWriter
		StatusCode int
		LogMessage string
	}
)

// NewApp create a new updater app
func NewApp(config *Config, clientset *kubernetes.Clientset) *App {
	// create counter and initialize
	counter := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bitsbeats_updater_deployment",
		Help: "increased whenever a update is triggered",
	}, []string{"action"})
	counter.With(prometheus.Labels{"action": "updated"}).Add(0)
	counter.With(prometheus.Labels{"action": "skipped"}).Add(0)

	return &App{
		Config:    config,
		Clientset: clientset,
		Counter:   counter,
	}
}

// Handle provide a http.HandleFunc to validate requests and trigger the patch
func (a *App) Handle(w http.ResponseWriter, r *http.Request) {
	// read http headers
	var token string
	if tokenList, ok := r.Header["Token"]; ok {
		token = tokenList[0]
	} else {
		w.(*AppResponseWriter).Abort(http.StatusForbidden, "token header missing")
		return
	}

	// verify token
	if subtle.ConstantTimeCompare([]byte(token), a.Config.Token) != 1 {
		w.(*AppResponseWriter).Abort(http.StatusForbidden, "invalid token")
		return
	}

	// patch deployment
	t := time.Now().Format(time.RFC3339)
	updating, err := a.Patch(t)
	if err != nil {
		msg := fmt.Sprintf("unable to update deployment: %s", err)
		w.(*AppResponseWriter).Abort(http.StatusInternalServerError, msg)
		return
	}
	if updating {
		msg := fmt.Sprintf("updated deployment: rolling out new version %s", t)
		w.(*AppResponseWriter).Ok(msg)
		a.Counter.With(prometheus.Labels{"action": "updated"}).Inc()
	} else {
		w.(*AppResponseWriter).Ok("updated deployment: no change detected")
		a.Counter.With(prometheus.Labels{"action": "skipped"}).Inc()
	}
}

// Middleware is a http middleware with extended logging
func (a *App) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		w = &AppResponseWriter{w, 200, ""}
		next.ServeHTTP(w, r)
		remote := r.RemoteAddr
		if forwarded, ok := r.Header["X-Forwarded-For"]; ok {
			remote = forwarded[0]
		}
		msg := fmt.Sprintf(
			"%d %s %fs %s %s",
			w.(*AppResponseWriter).StatusCode, remote,
			time.Since(t).Seconds(), r.Method, r.URL,
		)
		if w.(*AppResponseWriter).LogMessage != "" {
			msg += fmt.Sprintf(" - %q", w.(*AppResponseWriter).LogMessage)
		}
		log.Print(msg)
	})
}

// Patch patches a Deployment with an updater annotation to tigger a deployment
func (a *App) Patch(annotationValue string) (updating bool, err error) {
	template := `
{    
  "spec": {
    "template": {
      "metadata": {
        "annotations": {
          "thobits.com/updater": %q
        }                                                          
      }
    }
  }
}`
	resp, err := a.Clientset.AppsV1().Deployments(a.Config.Namespace).Patch(
		a.Config.Deployment,
		types.StrategicMergePatchType,
		[]byte(fmt.Sprintf(template, annotationValue)))
	updating = false
	if resp.Status.ObservedGeneration != resp.ObjectMeta.Generation {
		updating = true
	}
	return
}

// WriteHeader wraps ResponseWriters WriteHeader
func (w *AppResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.StatusCode = statusCode
}

// Abort defines the http errorcode an the message for the logger
func (w *AppResponseWriter) Abort(statusCode int, message string) {
	w.WriteHeader(statusCode)
	w.StatusCode = statusCode
	w.LogMessage = message
}

// Ok defines the message for the logger
func (w *AppResponseWriter) Ok(message string) {
	w.LogMessage = message
}
