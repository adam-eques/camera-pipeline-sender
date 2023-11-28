package vidoestreamsender

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"strconv"
	"strings"

	// encoders "github.com/acentior/camera-pipeline-sender/internal/encoders"
	"github.com/acentior/camera-pipeline-sender/internal/encoders"
	"github.com/acentior/camera-pipeline-sender/internal/signaling"
	"github.com/acentior/camera-pipeline-sender/pkg/size"
	"github.com/google/uuid"

	"github.com/nfnt/resize"
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/webrtc/v3"
)

var logger *log.Logger

func init() {
	logger = log.New(log.Writer(), "[streamSender]", log.LstdFlags)
}

type VideoStreamSender struct {
	sgl          *signaling.Signaling
	peerConn     *webrtc.PeerConnection
	webrtcConfig *webrtc.Configuration
	camCapturer  *CameraCapturer
	encService   *encoders.EncoderService
	streamer     *rtcStreamer
}

func (vss *VideoStreamSender) Init(websocktUrl string, stunUrl string) error {
	s := signaling.Signaling{}
	if err := s.Init(websocktUrl); err != nil {
		return err
	}

	// Init webrtc configuration
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

	// Init camera capturer
	cc, err := CreateCameraCapturer(640, 480, 60)
	if err != nil {
		return err
	}

	vss.sgl = &s
	vss.webrtcConfig = &peerConConfig
	vss.camCapturer = cc
	return nil
}

func (vss *VideoStreamSender) Run() error {
	defer vss.sgl.Close()

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

			offer := webrtc.SessionDescription{}
			fmt.Printf("offer: {%v}", offStr)
			decodeOffer(offStr, &offer)

			// enc := &encoders.EncoderService{}
			// webrtcCodec, encCodec, err := findBestCodec(&offer, enc, "42e01f")
			encCodec := encoders.H264Codec
			webrtcCodec := &webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType: webrtc.MimeTypeH264,
				},
			}

			if err != nil {
				panic(err)
			}
			mediaEngine := webrtc.MediaEngine{}
			mediaEngine.RegisterCodec(*webrtcCodec, webrtc.RTPCodecTypeVideo)
			api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))
			peerConnection, err := api.NewPeerConnection(*vss.webrtcConfig)
			if err != nil {
				panic(err)
			}
			vss.peerConn = peerConnection
			track, err := webrtc.NewTrackLocalStaticSample(
				webrtcCodec.RTPCodecCapability,
				"camera-video",
				uuid.New().String(),
			)
			if err != nil {
				panic(err)
			}

			logger.Printf("Using codec %s (%d) %s", webrtcCodec.MimeType, webrtcCodec.PayloadType, webrtcCodec.SDPFmtpLine)

			direction, err := getTrackDirection(&offer)
			if err != nil {
				return err
			}

			if direction == webrtc.RTPTransceiverDirectionSendrecv {
				_, err = peerConnection.AddTrack(track)
				if err != nil {
					panic(err)
				}
				logger.Println("Direction: RTPTransceiverDirectionSendrecv")
			} else if direction == webrtc.RTPTransceiverDirectionRecvonly {
				_, err = peerConnection.AddTransceiverFromTrack(track, webrtc.RtpTransceiverInit{
					Direction: webrtc.RTPTransceiverDirectionSendonly,
				})
				if err != nil {
					panic(err)
				}
				logger.Println("Direction: RTPTransceiverDirectionSendonly")
			} else {
				logger.Fatalln("Unsupported transceiver direction")
			}

			// Set the remote SessionDescription
			if err = peerConnection.SetRemoteDescription(offer); err != nil {
				panic(err)
			}

			// Create a encoder
			sourceSize := vss.camCapturer.Size()
			logger.Printf("encCodec: %+v\nwidth: %+v\nheight: %+v\nfps: %+v\n", encCodec, sourceSize.Width, sourceSize.Height, vss.camCapturer.Fps())
			encoder, err := vss.encService.NewEncoder(encCodec, sourceSize, vss.camCapturer.Fps())

			logger.Println("encoder start: ============")
			logger.Println(encoder)
			logger.Println("encoder end: ============")
			if err != nil {
				panic(err)
			}

			size, err := encoder.VideoSize()
			if err != nil {
				return err
			}

			streamer := newRTCStreamer(track, vss.camCapturer, &encoder, size)
			vss.streamer = streamer

			// Set the handler for ICE connection state
			// This will notify you when the peer has connected/disconnected
			peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
				if connectionState == webrtc.ICEConnectionStateConnected {
					vss.start()
				}
				if connectionState == webrtc.ICEConnectionStateDisconnected {
					vss.Stop()
				}
				logger.Printf("Connection State has changed %s \n", connectionState.String())
			})

			// Set the handler for Peer connection state
			// This will notify you when the peer has connected/disconnected
			peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
				logger.Printf("Peer Connection State has changed: %s\n", s.String())

				if s == webrtc.PeerConnectionStateFailed {
					// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
					// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
					// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
					logger.Println("Peer Connection has gone to failed exiting")
					os.Exit(0)
				}
			})

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

func (vss *VideoStreamSender) start() {
	vss.streamer.start()
}

func (vss *VideoStreamSender) Stop() error {
	if vss.streamer != nil {
		vss.streamer.Close()
	}

	if vss.peerConn != nil {
		return vss.peerConn.Close()
	}
	return nil
}

func resizeImage(src *image.RGBA, target size.Size) *image.RGBA {
	return resize.Resize(uint(target.Width), uint(target.Height), src, resize.Lanczos3).(*image.RGBA)
}

func imgToRGPA(img image.Image) *image.RGBA {
	rgbaImg := image.NewRGBA(img.Bounds())
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgbaImg
}

func findBestCodec(sdp *webrtc.SessionDescription, encService encoders.Service, h264Profile string) (*webrtc.RTPCodecParameters, encoders.VideoCodec, error) {
	sdpInfo, err := sdp.Unmarshal()
	if err != nil {
		return nil, encoders.NoCodec, err
	}
	var h264Codec *webrtc.RTPCodecParameters
	var vp8Codec *webrtc.RTPCodecParameters
	for _, md := range sdpInfo.MediaDescriptions {
		for _, format := range md.MediaName.Formats {
			intPt, err := strconv.Atoi(format)
			if err != nil {
				return nil, encoders.NoCodec, fmt.Errorf("Can't find codec for %d", 0)
			}
			payloadType := uint8(intPt)
			sdpCodec, err := sdpInfo.GetCodecForPayloadType(payloadType)
			if err != nil {
				return nil, encoders.NoCodec, fmt.Errorf("Can't find codec for %d", payloadType)
			}

			logger.Printf("CodecName: %v", sdpCodec.Name)
			if sdpCodec.Name == "H264" && h264Codec == nil {
				packetSupport := strings.Contains(sdpCodec.Fmtp, "packetization-mode=1")
				supportsProfile := strings.Contains(sdpCodec.Fmtp, fmt.Sprintf("profile-level-id=%s", h264Profile))
				logger.Printf("Fmtp: %v", sdpCodec.Fmtp)
				logger.Printf("\npacketSupport: %v\nsupportsProfile: %v", packetSupport, supportsProfile)
				if packetSupport && supportsProfile {
					h264Codec = &webrtc.RTPCodecParameters{
						RTPCodecCapability: webrtc.RTPCodecCapability{
							MimeType:    webrtc.MimeTypeH264,
							ClockRate:   sdpCodec.ClockRate,
							SDPFmtpLine: sdpCodec.Fmtp,
						},
						PayloadType: webrtc.PayloadType(sdpCodec.PayloadType),
					}
				}
			} else if sdpCodec.Name == "VP8" && vp8Codec == nil {
				// vp8Codec = webrtc.NewRTPVP8Codec(payloadType, sdpCodec.ClockRate)
				vp8Codec = &webrtc.RTPCodecParameters{
					RTPCodecCapability: webrtc.RTPCodecCapability{
						MimeType:    webrtc.MimeTypeVP8,
						ClockRate:   sdpCodec.ClockRate,
						SDPFmtpLine: sdpCodec.Fmtp,
					},
					PayloadType: webrtc.PayloadType(sdpCodec.PayloadType),
				}
			}
		}
	}
	if vp8Codec != nil && encService.Supports(encoders.VP8Codec) {
		return vp8Codec, encoders.VP8Codec, nil
	}
	if h264Codec != nil && encService.Supports(encoders.H264Codec) {
		return h264Codec, encoders.H264Codec, nil
	}
	return nil, encoders.NoCodec, fmt.Errorf("Couldn't find a matching codec")
}

func getTrackDirection(sdp *webrtc.SessionDescription) (webrtc.RTPTransceiverDirection, error) {
	sdpInfo, err := sdp.Unmarshal()
	if err != nil {
		return webrtc.RTPTransceiverDirectionInactive, err
	}
	for _, mediaDesc := range sdpInfo.MediaDescriptions {
		if mediaDesc.MediaName.Media == string(webrtc.MediaKindVideo) {
			if _, recvOnly := mediaDesc.Attribute("recvonly"); recvOnly {
				return webrtc.RTPTransceiverDirectionRecvonly, err
			} else if _, sendRecv := mediaDesc.Attribute("sendrecv"); sendRecv {
				return webrtc.RTPTransceiverDirectionSendrecv, err
			}
		}
	}
	return webrtc.RTPTransceiverDirectionInactive, nil
}
