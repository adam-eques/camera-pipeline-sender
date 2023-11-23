package sender

import (
	"github.com/pion/mediadevices"
	_ "github.com/pion/mediadevices/pkg/driver/camera" // This is required to register camera adapter
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
)

type CameraVideoStreamSender struct {
	stream     *mediadevices.MediaStream
	videoTrack *mediadevices.VideoTrack
}

func (sender *CameraVideoStreamSender) ConnectCamera() error {
	mediaStream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			c.FrameFormat = prop.FrameFormatOneOf{frame.FormatI420, frame.FormatYUY2}
			c.Width = prop.Int(640)
			c.Height = prop.Int(480)
		},
	})
	if err != nil {
		return err
	}
	sender.stream = &mediaStream
	track := mediaStream.GetVideoTracks()[0]
	videoTrack := track.(*mediadevices.VideoTrack)
	sender.videoTrack = videoTrack
	return nil
}

func (sender *CameraVideoStreamSender) IsCameraConnected() bool {
	return sender.stream != nil && sender.videoTrack != nil
}
