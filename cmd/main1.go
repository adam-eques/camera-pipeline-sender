package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/acentior/camera-pipeline-sender/internal/signal"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
)

const (
	audioFileName = "output.ogg"
	videoFileName = "output.h264"
)

func main1() {
	// Assert that we have an audio or video file
	_, err := os.Stat(videoFileName)
	haveVideoFile := !os.IsNotExist(err)

	_, err = os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile && !haveVideoFile {
		panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	if haveVideoFile {
		// Create a video track
		videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		rtpSender, videoTrackErr := peerConnection.AddTrack(videoTrack)
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		nextVideoSampleTime := time.Now()
		timePerFrame := time.Millisecond * 33 // 30fps = 1000ms/30frames = 33.3ms

		go func() {
			file, h264Err := os.Open(videoFileName)
			if h264Err != nil {
				panic(h264Err)
			}

			h264, h264Err := h264reader.NewReader(file)
			if h264Err != nil {
				panic(h264Err)
			}

			// Wait for connection established
			<-iceConnectedCtx.Done()

			count := 0
			const COUNT_MAX = 20

			for {
				nal, h264Err := h264.NextNAL()
				if h264Err == io.EOF {
					fmt.Printf("All video frames parsed and sent")
					count += 1
					if count >= COUNT_MAX {
						os.Exit(0)
					} else {
						h264, h264Err = h264reader.NewReader(file)
						if h264Err != nil {
							panic(h264Err)
						}
					}
				}
				if h264Err != nil {
					panic(h264Err)
				}

				// Golang's time.Sleep() is not precise enough for a consistent audio and video stream
				// (see https://github.com/golang/go/issues/44343). Therefore, don't use an absolute
				// sleep, but instead calculate the remaining sleep duration using wall clock time.
				// The packets still will not be perfectly timed, but the error will average out to the point
				// where the receiver's jitter buffer can compensate.
				nextVideoSampleTime = nextVideoSampleTime.Add(timePerFrame)
				sleepDuration := nextVideoSampleTime.Sub(time.Now())
				if sleepDuration > 0 {
					time.Sleep(sleepDuration)
				}

				if h264Err = videoTrack.WriteSample(media.Sample{Data: nal.Data, Duration: time.Second}); h264Err != nil {
					panic(h264Err)
				}
			}
		}()
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	// Block forever
	select {}
}
