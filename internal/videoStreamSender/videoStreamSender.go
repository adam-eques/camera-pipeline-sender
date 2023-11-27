package vidoestreamsender

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/acentior/camera-pipeline-sender/internal/signaling"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
)

var logger *log.Logger

func init() {
	logger = log.New(log.Writer(), "[streamSender]", log.LstdFlags)
}

type VideoStreamSender struct {
	sgl      *signaling.Signaling
	peerConn *webrtc.PeerConnection
}

func (vss *VideoStreamSender) Init(websocktUrl string, stunUrl string) error {
	s := signaling.Signaling{}
	if err := s.Init(websocktUrl); err != nil {
		return err
	}

	peerConConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{stunUrl},
			},
		},
	}
	if stunUrl == "" {
		peerConConfig = webrtc.Configuration{}
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(peerConConfig)
	if err != nil {
		return err
	}

	vss.peerConn = peerConnection
	vss.sgl = &s
	return nil
}

func (vss *VideoStreamSender) Run(videoFileName string) error {
	_, err := os.Stat(videoFileName)
	if err != nil {
		return err
	}
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	defer vss.sgl.Close()
	defer vss.peerConn.Close()

	vss.sgl.SendMsg(&signaling.WsMsg{
		Sender: true,
		WSType: signaling.CONNECTED,
	})

	for {
		message, err := vss.sgl.ReadMsg()
		if err != nil {
			log.Fatalf("Failed to read message from websocket {%v}", err)
		}
		logger.Printf("Received: {%v}", *message)
		switch message.WSType {
		case signaling.CONNECTED:
			if message.Data == "double streamer" {
				break
			} else {
			}
			break
		case signaling.SDP:
			offStr := message.SDP
			fmt.Printf("offer received {%v}", offStr)
			peerConnection := vss.peerConn
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
				logger.Printf("Start stream sending")
				logger.Printf("Start stream sending")

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

					logger.Printf("videoTrack: {%v}", videoTrack)
					if h264Err = videoTrack.WriteSample(media.Sample{Data: nal.Data, Duration: time.Second}); h264Err != nil {
						panic(h264Err)
					}
				}
			}()

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

			// Wait for the offer to be received
			offer := webrtc.SessionDescription{}
			fmt.Printf("offer: {%v}", offStr)
			decodeOffer(offStr, &offer)

			// Set the remote SessionDescription
			if err = peerConnection.SetRemoteDescription(offer); err != nil {
				panic(err)
			}

			// Create answer
			answer, err := peerConnection.CreateAnswer(nil)
			if err != nil {
				panic(err)
			}

			// send the answer in base64
			vss.sgl.SendMsg(&signaling.WsMsg{
				Sender: true,
				WSType: signaling.SDP,
				SDP:    encodeOffer(answer),
			})

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

			break
		}
	}

	return nil
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func decodeOffer(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	fmt.Printf("offer: {%v}", b)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		panic(err)
	}
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func encodeOffer(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

func (vss *VideoStreamSender) PeerClose() error {
	if vss != nil {
		if err := vss.peerConn.Close(); err != nil {
			return err
		}
	}
	return nil
}
