package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	docopt "github.com/docopt/docopt-go"
	"github.com/prometheus/client_golang/prometheus"
)

func getConfig() (DarknetDConfig, error) {
	c := DarknetDConfig{}
	args, err := docopt.Parse(usage, nil, true, version, false)
	if err != nil {
		return c, fmt.Errorf("Error parsing args: %s", err.Error())
	}
	c.capDir = args["--capture-dir"].(string)
	c.capFile = args["--capture-file"].(string)
	c.listenAddr = args["--listen-addr"].(string)
	c.archiveDir = args["--archive-dir"].(string)
	c.archiveFiles, err = strconv.Atoi(args["--archive-files"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --archive-files: %s", err.Error())
	}
	timeoutMsec, err := strconv.Atoi(args["--start-timeout"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --detect-timeout: %s", err.Error())
	}
	c.darknetStartTimeout = time.Duration(timeoutMsec) * time.Millisecond
	timeoutMsec, err = strconv.Atoi(args["--detect-timeout"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --detect-timeout: %s", err.Error())
	}
	c.darknetDetectTimeout = time.Duration(timeoutMsec) * time.Millisecond
	delayMsec, err := strconv.Atoi(args["--detect-delay"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --detect-delay: %s", err.Error())
	}
	c.darknetDetectDelay = time.Duration(delayMsec) * time.Millisecond
	c.darknetDir = args["--darknet-dir"].(string)
	c.darknetDataFile = args["--darknet-data"].(string)
	c.modelConfigFile = args["--model-config"].(string)
	c.modelWeightsFile = args["--model-weights"].(string)
	return c, nil
}

func startArchiveManager(archiveDir string, archiveFiles int, cleanedUpFiles prometheus.Counter, cleanUpErrors *prometheus.CounterVec) error {
	go func() {
		cleanTick := time.NewTicker(time.Millisecond * 10000)
		defer cleanTick.Stop()
		for {
			select {
			case <-cleanTick.C:
				files, err := ioutil.ReadDir(archiveDir)
				if err != nil {
					cleanUpErrors.WithLabelValues("ReadDir").Add(1)
					log.Printf("Cleanup ReadDir error: %s", err)
					break
				}
				if len(files) > archiveFiles {
					if err := cleanupFiles(archiveDir, len(files)-archiveFiles, files); err != nil {
						log.Printf("Cleanup error: %s", err)
						cleanUpErrors.WithLabelValues("cleanupFiles").Add(1)
					} else {
						cleanedUpFiles.Add(float64(len(files) - archiveFiles))
					}
				}
			}
		}
	}()
	return nil
}

func cleanupFiles(archiveDir string, numToCleanup int, files []os.FileInfo) error {
	fs := map[string]os.FileInfo{}
	for _, f := range files {
		fs[f.Name()] = f
	}
	for i := 0; i < numToCleanup; i++ {
		oldest := findOldest(fs)
		if err := os.Remove(filepath.Join(archiveDir, oldest.Name())); err != nil {
			return err
		}
		delete(fs, oldest.Name())
	}
	return nil
}

func findOldest(files map[string]os.FileInfo) os.FileInfo {
	first := true
	var oldest os.FileInfo
	for _, f := range files {
		if first {
			first = false
			oldest = f
			continue
		}
		if f.ModTime().Before(oldest.ModTime()) {
			oldest = f
		}
	}
	return oldest
}

func findNewest(archiveDir string) (os.FileInfo, error) {
	var newest os.FileInfo
	files, err := ioutil.ReadDir(archiveDir)
	if err != nil {
		return newest, err
	}
	first := true
	for _, f := range files {
		if first {
			first = false
			newest = f
			continue
		}
		if !strings.HasSuffix(f.Name(), ".jpg") {
			continue
		}
		if strings.HasPrefix(f.Name(), "predictions_") {
			continue
		}
		if f.ModTime().After(newest.ModTime()) {
			newest = f
		}
	}
	if newest == nil || len(newest.Name()) < 1 {
		return newest, fmt.Errorf("No image file found")
	}
	return newest, nil
}
