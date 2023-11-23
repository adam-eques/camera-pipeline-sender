package main

import (
	"log"

	"github.com/acentior/camera-pipeline-sender/pkg/sender"
)

func main() {
	log.Printf("camera-pipeline-sender")

	// s := sender.CameraVideoStreamSender{}
	// s := sender.CameraVideoStreamSender{}
	s := sender.CameraVideoStreamSender{}
	s.ConnectCamera()
}
