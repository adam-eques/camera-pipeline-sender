package vidoestreamsender

import (
	"image"
	"log"
	"time"

	"github.com/acentior/camera-pipeline-sender/internal/encoders"
	"github.com/acentior/camera-pipeline-sender/pkg/size"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

type rtcStreamer struct {
	track       *webrtc.TrackLocalStaticSample
	stop        chan struct{}
	encoder     *encoders.Encoder
	size        size.Size
	camCapturer *CameraCapturer
}

func init() {
	logger = log.New(log.Writer(), "[videoStreamer/rtcStreamer]", log.LstdFlags)
}

func newRTCStreamer(track *webrtc.TrackLocalStaticSample, capturer *CameraCapturer, encoder *encoders.Encoder, size size.Size) *rtcStreamer {
	return &rtcStreamer{
		track:       track,
		stop:        make(chan struct{}),
		encoder:     encoder,
		size:        size,
		camCapturer: capturer,
	}
}

func (s *rtcStreamer) start() {
	go func() {
		capturer := *s.camCapturer
		capturer.Start()
		frames := capturer.Frames()
		for {
			select {
			case <-s.stop:
				capturer.Stop()
				return
			case frame := <-frames:
				err := s.stream(frame)
				if err != nil {
					logger.Printf("Streamer: %v\n", err)
					return
				}
			}
		}
	}()
}

func (s *rtcStreamer) stream(frame *image.RGBA) error {
	resized := resizeImage(frame, s.size)
	payload, err := (*s.encoder).Encode(resized)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}
	delta := time.Duration(1000/s.camCapturer.Fps()) * time.Millisecond
	return s.track.WriteSample(media.Sample{
		Data:      payload,
		Timestamp: time.Now(),
		Duration:  delta,
	})
}

func (s *rtcStreamer) Close() {
	close(s.stop)
}
