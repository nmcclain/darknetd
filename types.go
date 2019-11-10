package main

import (
	"io"
	"sync"
	"time"

	"github.com/zfjagann/golang-ring"
)

type DarknetD struct {
	config  DarknetDConfig
	metrics Metrics

	detections    *ring.Ring
	detectionsmtx sync.RWMutex

	cmdin  io.WriteCloser
	cmdout io.ReadCloser
	cmdmtx sync.Mutex
}

type DarknetDConfig struct {
	capDir               string
	capFile              string
	listenAddr           string
	archiveDir           string
	archiveFiles         int
	darknetStartTimeout  time.Duration
	darknetDetectTimeout time.Duration
	darknetDetectDelay   time.Duration
	darknetDir           string
	darknetDataFile      string
	modelConfigFile      string
	modelWeightsFile     string
}

type DarknetJobResult struct {
	status        DarknetJobStatus
	darknetResult DarknetResult
}

type DarknetJobStatus int

const (
	DARKNET_STOPPED DarknetJobStatus = iota
	DARKNET_RUNNING
)

type DarknetResult struct {
	Image      string
	PredImage  string
	ImageTime  time.Time
	PredTime   time.Time
	TimeDetect float64
	TimeTotal  float64
	Objects    []Object
}

type Object struct {
	Class string
	Prob  int
	Left  int
	Right int
	Top   int
	Bot   int
}
