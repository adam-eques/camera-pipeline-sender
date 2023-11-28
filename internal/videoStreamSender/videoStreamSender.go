package vidoestreamsender

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"time"

	// encoders "github.com/acentior/camera-pipeline-sender/internal/encoders"
	"github.com/acentior/camera-pipeline-sender/internal/encoders"
	"github.com/acentior/camera-pipeline-sender/internal/signaling"

	"github.com/nfnt/resize"
	"github.com/pion/mediadevices"
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
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

			_, videoTrackErr = peerConnection.AddTrack(videoTrack)
			if videoTrackErr != nil {
				panic(videoTrackErr)
			}

			// Read incoming RTCP packets
			// Before these packets are returned they are processed by interceptors. For things
			// like NACK this needs to be called.

			go func() {
				size := image.Point{640, 480}
				fps := 12
				h264Encoder, err := encoders.NewH264Encoder(size, fps)
				if err != nil {
					log.Panic("Failed to get h264encoder", err)
				}
				rSize, err := h264Encoder.VideoSize()
				if err != nil {
					log.Panic("Failed to get target size", err)
				}

				stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
					Video: func(mtc *mediadevices.MediaTrackConstraints) {},
				})
				if err != nil {
					log.Panic("Failed to get camera", err)
				}

				logger.Printf("rSize %v", rSize)
				vTrack := stream.GetVideoTracks()[0]
				frameReader := vTrack.(*mediadevices.VideoTrack).NewReader(true)

				// Wait for connection established
				<-iceConnectedCtx.Done()
				logger.Printf("Start stream sending")
				interval := time.Second / time.Duration(fps)
				ticker := time.NewTicker(interval)
				for range ticker.C {
					imgFrame, _, err := frameReader.Read()
					if err != nil {
						log.Panic("Failed to get image frame from camera", err)
					}

					rgbaImage := imgToRGPA(imgFrame)
					resized := resizeImage(rgbaImage, rSize)
					encodedImage, err := h264Encoder.Encode(resized)
					if err != nil {
						logger.Printf("encode image error")
						continue
					}
					if h264Err := videoTrack.WriteSample(media.Sample{Data: encodedImage, Timestamp: time.Now()}); h264Err != nil {
						panic(h264Err)
					}
					logger.Printf("captured")
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

func resizeImage(src *image.RGBA, target image.Point) *image.RGBA {
	return resize.Resize(uint(target.X), uint(target.Y), src, resize.Lanczos3).(*image.RGBA)
}

func imgToRGPA(img image.Image) *image.RGBA {
	rgbaImg := image.NewRGBA(img.Bounds())
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgbaImg
}
