package vidoestreamsender

import (
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/acentior/camera-pipeline-sender/pkg/size"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
)

type CameraCapturer struct {
	fps          int
	agentNum     int
	frames       chan *image.RGBA
	stop         chan struct{}
	agentAdded   chan struct{}
	agentRemoved chan struct{}
	frameReader  *video.Reader
	size         size.Size
}

func CreateCameraCapturer(width int, height int, fps int) (*CameraCapturer, error) {
	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(mtc *mediadevices.MediaTrackConstraints) {
			mtc.Width = prop.Int(width)
			mtc.Height = prop.Int(height)
		},
	})
	if err != nil {
		return nil, err
	}

	vSize := size.Size{Width: width, Height: height}

	videoTracks := stream.GetVideoTracks()
	if len(videoTracks) < 1 {
		return nil, errors.New("Failed to get proper video track from camera")
	}

	vTrack := videoTracks[0].(*mediadevices.VideoTrack)
	freader := vTrack.NewReader(true)
	// img, _, err := freader.Read()
	// bounds := img.Bounds()
	// vSize := size.Size{Width: bounds.Dx(), Height: bounds.Dy()}
	// if err != nil {
	// 	return nil, err
	// }

	return &CameraCapturer{
		fps:          fps,
		agentNum:     0,
		frames:       make(chan *image.RGBA),
		stop:         make(chan struct{}),
		agentAdded:   make(chan struct{}),
		agentRemoved: make(chan struct{}),
		frameReader:  &freader,
		size:         vSize,
	}, nil
}

// Start initiates the screen capture loop
func (cc *CameraCapturer) Start() {
	fmt.Println("capturer started")
	delta := time.Duration(1000/cc.fps) * time.Millisecond
	go func() {
		for {
			startedAt := time.Now()
			select {
			case <-cc.stop:
				fmt.Println("Close cam capturer")
				close(cc.frames)
				return
			case <-cc.agentAdded:
				cc.agentNum += 1
				fmt.Printf("new agent added %v\n", cc.agentNum)
			case <-cc.agentRemoved:
				cc.agentNum -= 1
				fmt.Printf("new agent removed %v\n", cc.agentNum)
			default:
				img, _, err := (*cc.frameReader).Read()
				if err != nil {
					fmt.Printf("Error while read cam: %v\n", err)
					return
				}
				rgbaImage := imgToRGPA(img)
				for i := 0; i < cc.agentNum; i++ {
					cc.frames <- rgbaImage
				}
				ellapsed := time.Now().Sub(startedAt)
				sleepDuration := delta - ellapsed
				if sleepDuration > 0 {
					time.Sleep(sleepDuration)
				}
			}
		}
	}()
}

// Frames returns a channel that will receive an image stream
func (cc *CameraCapturer) Frames() <-chan *image.RGBA {
	return cc.frames
}

// Stop sends a stop signal to the capture loop
func (cc *CameraCapturer) Stop() {
	close(cc.stop)
}

// Fps returns the frames per sec. we're capturing
func (cc *CameraCapturer) Fps() int {
	return cc.fps
}

// Get size (width and height of the captured image)
func (cc *CameraCapturer) Size() size.Size {
	return cc.size
}

func (cc *CameraCapturer) AgentAdded() {
	cc.agentAdded <- struct{}{}
}

func (cc *CameraCapturer) AgentRemoved() {
	cc.agentRemoved <- struct{}{}
}
