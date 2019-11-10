package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	ApiRequests    *prometheus.CounterVec
	ApiErrors      *prometheus.CounterVec
	CleanedUpFiles prometheus.Counter
	CleanUpErrors  *prometheus.CounterVec
	JobErrors      prometheus.Counter
	Detections     prometheus.Counter
	PredTime       prometheus.Histogram
	TotalTime      prometheus.Histogram
}

func setupMetrics() Metrics {
	m := Metrics{}
	m.ApiRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "darknetd",
		Name:      "api_requests",
		Help:      "API requests.",
	}, []string{"handler"})
	m.ApiErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "darknetd",
		Name:      "api_errors",
		Help:      "API errors.",
	}, []string{"handler", "error"})
	m.CleanUpErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "darknetd",
		Name:      "cleanup_errors",
		Help:      "Image cleanup errors.",
	}, []string{"error"})
	m.CleanedUpFiles = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "darknetd",
		Name:      "cleanup_files",
		Help:      "Image files cleaned up.",
	})
	m.JobErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "darknetd",
		Name:      "detection_errors",
		Help:      "Darknet detection errors.",
	})
	m.Detections = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "darknetd",
		Name:      "detections",
		Help:      "Darknet successful detection jobs.",
	})
	m.PredTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "darknetd",
		Name:      "prediction_sec",
		Help:      "Darknet prediction time sec",
		Buckets:   []float64{.001, .025, .05, .1, .25, .5, .6, .7, .8, .9, 1, 1.5, 2},
	})
	m.TotalTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "darknetd",
		Name:      "total_sec",
		Help:      "Total job time sec",
		Buckets:   []float64{.001, .025, .05, .1, .25, .5, .6, .7, .8, .9, 1, 1.5, 2},
	})
	prometheus.MustRegister(
		m.ApiRequests,
		m.ApiErrors,
		m.CleanUpErrors,
		m.CleanedUpFiles,
		m.Detections,
		m.JobErrors,
		m.PredTime,
		m.TotalTime,
	)
	return m
}
