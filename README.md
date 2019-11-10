# darknetd: run darknet as a service, with a REST API
`darknetd` provides a service and API wrapper around the [darknet](https://pjreddie.com/darknet/) command-line tool.  

It allows you to efficiently run darknet YOLO image recognition on an "edge device", like [Raspberry Pi](https://www.raspberrypi.org/) or [Jetson Nano](https://www.nvidia.com/en-us/autonomous-machines/embedded-systems/jetson-nano/), and access the results via a [simple REST API](#api).

## Features
* Runs `darknet` as a service, avoiding startup time spent building the network and loading weights.
* Provides an API for viewing recent object detections, including access to raw source and prediction images.
* Works with external image capture tool (such as raspistill), allowing fine-tuning of camera settings.
* Archives recent darknet predictions.jpg images for review.
* Automatically deletes old images, ensuring your SD card/disk doesn't fill up.
* Exposes performance metrics in prometheus format.

## Motivation
There are lots of great machine vision tools, but it's challenging to get them "into production" on remote edge devices.  Performing the object detection on a commodity edge device offers many benefits:

* Images stay on the edge device (unless specifically requested), providing detection data without compromising privacy.
* Bandwidth usage is minimal for object detection API responses.
* The YOLOv3-tiny model can perform detection at about ~2 FPS on a Raspberry Pi and ~20 FPS on a Jetson Nano.
* Raspberry Pi4 Kit/Jetson Nano DevKit are $99 - 5MP camera module ~$20.

# Quick start

1. Download and build darknet.
   * IMPORTANT: darknetd depends on a [modified version of darknet](https://github.com/nmcclain/darknet-nnpack)!  This [small modification](https://github.com/nmcclain/darknet-nnpack/commit/9faadb17f6f14c2c1aefa578a7916e3e8a09950a) causes darknet to print bounding box information for detected objects.
   * We modified [darknet-nnpack](https://github.com/digitalbrain79/darknet-nnpack) because it is optimized for the Raspberry Pi - you could easily apply [this modification](https://github.com/nmcclain/darknet-nnpack/commit/9faadb17f6f14c2c1aefa578a7916e3e8a09950a) to the base darknet distribution instead.
   * `darknetd` assumes the `darknet` binary is at `/usr/local/darknet/darknet`.
   1. Install `darknet` per [the README](https://github.com/digitalbrain79/darknet-nnpack/blob/master/README.md).
   1. Move the `darknet` directory to `/usr/local/`: `mv darknet /usr/local/darknet`
   1. Download the pre-trained YOLOv3-tiny weights: `curl -o /usr/local/darknet/yolov3-tiny.weights https://pjreddie.com/media/files/yolov3-tiny.weights`
   1. Confirm `darknet` is working from the command-line: `cd /usr/local/darknet && ./darknet detect cfg/yolov3-tiny.cfg yolov3-tiny.weights data/dog.jpg`
1. Setup the `raspistill` service - `darknetd` works hand-in-hand with this service.
   1. [Get your camera working](https://projects.raspberrypi.org/en/projects/getting-started-with-picamera) and verify you can capture images with raspistill: `raspistill -o cam.jpg`.
   1. Install the [raspistill systemd file](etc/raspistill.service) at `/etc/systemd/system/raspistill.service`.
   1. Review the `raspistill.service` file and customize camera/capture options as desired.
   1. Create the image archive directory: `mkdir /tmp/cap`
   1. Configure the `raspistill` service to start at boot: `sudo systemctl enable raspistill`
   1. Start the `raspistill` service: `sudo systemctl start raspistill`
   1. Verify images are being stored in `/tmp/cap`
1. Install `darknetd`:
   1. [Download darknetd](../../releases) and install it at `/usr/local/sbin/darknetd`.
   1. Install the `darknetd.service` systemd file at `/etc/systemd/system/darknetd.service`.
   1. Configure `darknetd` to start at boot: `sudo systemctl enable darknetd`
   1. Start `darknetd`: `sudo systemctl start darknetd`

# Usage
Darknetd has sensible defaults, and supports the following options - be sure to set them in your `darknetd.service` file.

```shell
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
  --detect-timeout=<secs>     Darknet detection timeout [default: 10]
  --detect-delay=<msec>       Darknet delay between detections in msec [default: 500]
  --listen-addr=<addr:port>   Darknet detection timeout [default: 0.0.0.0:8081]
  --version                   Show version
  -h, --help                  Show this screen
```

* To use a custom model: `darknetd --darknet-data=cfg/YOUR.data --model-config=cfg/YOUR-MODEL.cfg --model-weights=YOUR-MODEL.weights`
* Note that on a Pi4, setting `--detect-delay` below 200 msec can cause significant CPU load.  The default of 500 is a reasonable balance of detection time and CPU usage.

# API

WARNING: The API provides no authentication and is *NOT* intended to be exposed direclty to a public network!

* `GET /objects` - returns JSON list of most recent predictions
* `GET /latest.jpg` - returns latest source image
* `GET /image/{imagename}.jpg` - returns source or prediction image (get imagename from `/objects` output)
* `GET /metrics` - returns performance metrics in prometheus format
* `GET /health` - returns `OK` if healthy

Sample API request (*note: returns up to 10 most recent detections*):
```shell
$ curl -s localhost:8081/objects
[
  {
    "Image": "image208725.jpg",
    "PredImage": "predictions_image208725.jpg",
    "ImageTime": "2019-09-17T16:10:58.313756895-06:00",
    "PredTime": "2019-09-17T16:10:59.812694026-06:00",
    "TimeDetect": 0.767813,
    "TimeTotal": 0.893044895,
    "Objects": [
      {
        "Class": "person",
        "Prob": 85,
        "Left": 365,
        "Right": 445,
        "Top": 314,
        "Bot": 413
      }
    ]
  }
]
$ curl -s localhost:8081/image/image189401.jpg -o src_image.jpg
$ curl -s localhost:8081/image/predictions_image189401.jpg -o pred_image.jpg
```
