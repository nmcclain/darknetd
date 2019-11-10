package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

const version = "1.0.0"

var usage = `darknetd: run darknet as a service, with a REST API

Usage:
  darknetd [options]
  darknetd -h --help
  darknetd --version

Options:
  --capture-dir=<path>        Directory containing captured image - see raspiconfig.service [default: /tmp/]
  --capture-file=<file>       Filename of captured image - see raspiconfig.service [default: cap.jpg]
  --archive-dir=<path>        Directory containing image archive - see raspiconfig.service [default: /tmp/cap]
  --archive-files=<file>      Number of images to retain in archive [default: 240]
  --darknet-dir=<path>        Directory containing darknet installation [default: /usr/local/darknet]
  --darknet-data=<file>       Darknet data file, relative to darknet-dir [default: cfg/coco.data]
  --model-config=<file>       Darknet model config file, relative to darknet-dir [default: cfg/yolov3-tiny.cfg]
  --model-weights=<file>      Darknet model weights file, relative to darknet-dir [default: yolov3-tiny.weights]
  --start-timeout=<msec>      Darknet startup & model load timeout in msec [default: 30000]
  --detect-timeout=<msec>     Darknet detection timeout in msec [default: 10000]
  --detect-delay=<msec>       Darknet delay between detections in msec [default: 500]
  --listen-addr=<addr:port>   API listen address:port [default: 0.0.0.0:8081]
  --version                   Show version
  -h, --help                  Show this screen
`

const (
	detectFilename      = "detect.jpg"
	darknetRestartDelay = time.Second * 5
)

func main() {
	log.Printf("Starting darknet")
	darknetConfig, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}
	var dd DarknetD
	darknetStatus := DARKNET_STOPPED // TODO: restart darknet upon process/IO failure
	for darknetStatus != DARKNET_RUNNING {
		dd, err = startDarknet(darknetConfig)
		if err != nil {
			log.Printf("Error starting darknet, trying again in %v: %s", darknetRestartDelay, err)
			time.Sleep(darknetRestartDelay)
			continue
		}
		darknetStatus = DARKNET_RUNNING
	}
	defer func() {
		dd.cmdout.Close()
		dd.cmdin.Close()
	}()
	log.Printf("Started darknet process")

	if err := startArchiveManager(
		dd.config.archiveDir,
		dd.config.archiveFiles,
		dd.metrics.CleanedUpFiles,
		dd.metrics.CleanUpErrors,
	); err != nil {
		log.Fatalf("startArchiveManager error %v", err)
	}
	if err := dd.startJobsManager(); err != nil {
		log.Fatalf("startJobsManager error %v", err)
	}

	log.Printf("Starting API on %s", dd.config.listenAddr)
	if err := dd.startAPI(dd.config.listenAddr); err != nil {
		log.Fatalf("Error starting API on %s: %v", dd.config.listenAddr, err)
	}
	log.Printf("Exiting")
}
