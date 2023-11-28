package vidoestreamsender

import (
	"errors"
	"image"
	"time"

	"github.com/acentior/camera-pipeline-sender/pkg/size"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
)

type CameraCapturer struct {
	fps         int
	frames      chan *image.RGBA
	stop        chan struct{}
	frameReader *video.Reader
	size        size.Size
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

	return &CameraCapturer{
		fps:         fps,
		frames:      make(chan *image.RGBA),
		stop:        make(chan struct{}),
		frameReader: &freader,
		size:        vSize,
	}, nil
}

// Start initiates the screen capture loop
func (cc *CameraCapturer) Start() {
	delta := time.Duration(1000/cc.fps) * time.Millisecond
	go func() {
		for {
			startedAt := time.Now()
			select {
			case <-cc.stop:
				close(cc.frames)
				return
			default:
				img, _, err := (*cc.frameReader).Read()
				if err != nil {
					return
				}
				rgbaImage := imgToRGPA(img)
				cc.frames <- rgbaImage
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
