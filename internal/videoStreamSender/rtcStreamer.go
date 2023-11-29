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
	tracks      []*webrtc.TrackLocalStaticSample
	stop        chan struct{}
	newTrack    chan *webrtc.TrackLocalStaticSample
	removeTrack chan *webrtc.TrackLocalStaticSample
	encoder     *encoders.Encoder
	size        size.Size
	camCapturer *CameraCapturer
}

func init() {
	logger = log.New(log.Writer(), "[videoStreamer/rtcStreamer]", log.LstdFlags)
}

func newRTCStreamer(tracks []*webrtc.TrackLocalStaticSample, capturer *CameraCapturer, encoder *encoders.Encoder, size size.Size) *rtcStreamer {
	return &rtcStreamer{
		tracks:      tracks,
		stop:        make(chan struct{}),
		newTrack:    make(chan *webrtc.TrackLocalStaticSample),
		removeTrack: make(chan *webrtc.TrackLocalStaticSample),
		encoder:     encoder,
		size:        size,
		camCapturer: capturer,
	}
}

func (s *rtcStreamer) start() {
	go func() {
		capturer := *s.camCapturer
		// capturer.agentAdded <- struct{}{}
		frames := capturer.Frames()
		for {
			select {
			case <-s.stop:
				// capturer.agentRemoved <- struct{}{}
				// logger.Println("completed streamer")
				return
			case newTrack := <-s.newTrack:
				s.tracks = append(s.tracks, newTrack)
			case track := <-s.removeTrack:
				tracks := []*webrtc.TrackLocalStaticSample{}
				for _, v := range s.tracks {
					if v.ID() != track.ID() {
						tracks = append(tracks, v)
					}
				}
				s.tracks = tracks
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
	for _, track := range s.tracks {
		err := track.WriteSample(media.Sample{
			Data:      payload,
			Timestamp: time.Now(),
			Duration:  delta,
		})
		if err != nil {
			logger.Printf("Sample is not written into the track %v", track.ID())
		}
	}
	return nil
}

func (s *rtcStreamer) Close() {
	close(s.stop)
}
