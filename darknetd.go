package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zfjagann/golang-ring"
)

func startDarknet(darknetConfig DarknetDConfig) (DarknetD, error) {
	dd := DarknetD{
		config:        darknetConfig,
		detectionsmtx: sync.RWMutex{},
		detections:    &ring.Ring{},
		cmdmtx:        sync.Mutex{},
	}
	dd.metrics = setupMetrics()
	dd.detections.SetCapacity(10)
	if err := os.Chdir(dd.config.darknetDir); err != nil {
		return dd, err
	}
	args := []string{"detector", "test", dd.config.darknetDataFile, dd.config.modelConfigFile, dd.config.modelWeightsFile}
	c := "./darknet"
	log.Printf("EXEC %s %s", c, strings.Join(args, " "))
	cmd := exec.Command(c, args...)
	cmderr, err := cmd.StderrPipe()
	if err != nil {
		return dd, err
	}
	dd.cmdout, err = cmd.StdoutPipe()
	if err != nil {
		return dd, err
	}
	dd.cmdin, err = cmd.StdinPipe()
	if err != nil {
		return dd, err
	}

	ready := make(chan bool)
	execErr := make(chan error)

	defer func() {
		cmderr.Close()
	}()

	go func(cmdout io.ReadCloser) {
		scanner := bufio.NewScanner(cmdout)
		scanner.Split(bufio.ScanWords)
		for ok := true; ok != false; ok = scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "Path:") {
				ready <- true
				return
			}
		}
		if scanner.Err() != nil {
			execErr <- fmt.Errorf("Error reading from darknet stdout on process start: %+v\n", scanner.Err())
		}
		return
	}(dd.cmdout)

	go func(cmderr io.ReadCloser) {
		reader := bufio.NewReader(cmderr)
		var err error
		err = nil
		for err == nil {
			e, err := reader.ReadString('\n') // ignore stderr - would be nice to show this on failure
			if err != nil {
				break
			}
			log.Printf("[stderr] %s\n", e)
		}
		if err != nil && err != io.EOF {
			log.Printf("Error reading from darknet stderr on process start: %+v", err)
			return
		}
		return
	}(cmderr)

	if err := cmd.Start(); err != nil {
		return dd, err
	}

	earlyExit := make(chan error)
	go func() { earlyExit <- cmd.Wait() }()

	select {
	case _ = <-ready:
		break
	case err := <-execErr:
		return dd, fmt.Errorf("Darknet stdout err on start: %s", err)
	case err := <-earlyExit:
		if err != nil {
			return dd, fmt.Errorf("Darknet start error: %s", err)
		}
		return dd, fmt.Errorf("Darknet exited on start")
	case <-time.After(dd.config.darknetStartTimeout):
		return dd, fmt.Errorf("Timed out starting darknet")
	}
	return dd, nil
}

func (dd *DarknetD) startJobsManager() error {
	go func() {
		for {
			lr, err := dd.handleJob(dd.config.archiveDir)
			if err != nil {
				log.Printf("Error handling job at %s: %s", dd.config.archiveDir, err)
				dd.metrics.JobErrors.Add(1)
				time.Sleep(dd.config.darknetDetectDelay)
				continue
			}
			dd.detections.Enqueue(lr)
			dd.metrics.Detections.Add(1)
			time.Sleep(dd.config.darknetDetectDelay)
		}
	}()
	return nil
}

func (dd *DarknetD) handleJob(srcDir string) (DarknetResult, error) {
	start := time.Now()
	dd.cmdmtx.Lock()
	defer dd.cmdmtx.Unlock()

	imgFile, err := findNewest(srcDir)
	if err != nil {
		return DarknetResult{}, err
	}
	imgTime := imgFile.ModTime()

	_ = os.Remove(filepath.Join(dd.config.capDir, detectFilename))
	if err := os.Symlink(filepath.Join(srcDir, imgFile.Name()), filepath.Join(dd.config.capDir, detectFilename)); err != nil {
		return DarknetResult{}, err
	}
	defer os.Remove(filepath.Join(dd.config.capDir, detectFilename))
	// log.Printf("calling darknet detect on %s", filepath.Join(dd.config.capDir, detectFilename))
	fmt.Fprintln(dd.cmdin, filepath.Join(dd.config.capDir, detectFilename))

	scanner := bufio.NewScanner(dd.cmdout)
	scanner.Split(bufio.ScanWords)

	words := []string{}
	for ok := scanner.Scan(); ok != false; ok = scanner.Scan() {
		words = append(words, scanner.Text())
		if scanner.Text() == "Path:" {
			break
		}
	}
	if scanner.Err() != nil {
		return DarknetResult{}, fmt.Errorf("Error reading from darknet stdout: %+v\t%v\n", scanner.Err(), words)
	}

	darknetResult, err := parseOutput(filepath.Join(dd.config.capDir, detectFilename), words)
	if err != nil {
		return DarknetResult{}, fmt.Errorf("Error parsing darknet output: %s\t%v", err, words)
	}
	darknetResult.Image = imgFile.Name()
	darknetResult.ImageTime = imgTime

	predImg, err := ioutil.ReadFile(filepath.Join(dd.config.darknetDir, "predictions.jpg"))
	if err != nil {
		return DarknetResult{}, err
	}
	predImgFile := fmt.Sprintf("predictions_%s", imgFile.Name())
	dst := filepath.Join(dd.config.archiveDir, predImgFile)
	err = ioutil.WriteFile(dst, predImg, 0644)
	if err != nil {
		return DarknetResult{}, err
	}
	darknetResult.PredImage = predImgFile
	darknetResult.PredTime = time.Now()
	darknetResult.TimeTotal = time.Since(start).Seconds()
	dd.metrics.Detections.Add(1)

	dd.metrics.PredTime.Observe(darknetResult.TimeDetect)
	dd.metrics.TotalTime.Observe(time.Since(start).Seconds())

	return darknetResult, nil
}

// parseOutput is pretty brittle, but works at least with https://github.com/nmcclain/darknet-nnpack 9faadb1
func parseOutput(imgFile string, words []string) (DarknetResult, error) {
	lr := DarknetResult{}

	for i := 0; i < len(words); i++ {
		switch words[i] {
		case "Predicted":
			i += 2 // "Predicted in"
			var err error
			lr.TimeDetect, err = strconv.ParseFloat(words[i], 64)
			if err != nil {
				log.Printf("Unexpected darknet output: %v: %s", words, err)
			}
			i++ // "seconds."
			unit := words[i]
			if unit == "milli-seconds." {
				lr.TimeDetect = lr.TimeDetect / 1000
			}
		case "CLASS":
			o := Object{}

			i++
			o.Class = words[i]
			i++

			var err error
			o.Prob, err = strconv.Atoi(words[i])
			if err != nil {
				log.Printf("Unexpected darknet output: %v: %s", words, err)
			}
			i += 2 // "BBOX"
			o.Left, err = strconv.Atoi(words[i])
			if err != nil {
				log.Printf("Unexpected darknet output: %v: %s", words, err)
			}
			i++
			o.Right, err = strconv.Atoi(words[i])
			if err != nil {
				log.Printf("Unexpected darknet output: %v: %s", words, err)
			}
			i++
			o.Top, err = strconv.Atoi(words[i])
			if err != nil {
				log.Printf("Unexpected darknet output: %v: %s", words, err)
			}
			i++
			o.Bot, err = strconv.Atoi(words[i])
			if err != nil {
				log.Printf("Unexpected darknet output: %v: %s", words, err)
			}
			lr.Objects = append(lr.Objects, o)
		case "Enter":
			break
		case "Image":
			break
		case "Path:":
			break
		case fmt.Sprintf("%s:", imgFile):
			break
		default:
			log.Printf("Unexpected darknet output: %v", words)
		}
	}
	return lr, nil
}
