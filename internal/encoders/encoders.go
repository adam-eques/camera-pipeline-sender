package encoders

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"github.com/acentior/camera-pipeline-sender/pkg/size"
	x264 "github.com/gen2brain/x264-go"
)

// H264Encoder h264 encoder
type H264Encoder struct {
	buffer   *bytes.Buffer
	encoder  *x264.Encoder
	realSize size.Size
}

const h264SupportedProfile = "3.1"

func newH264Encoder(size size.Size, frameRate int) (Encoder, error) {
	buffer := bytes.NewBuffer(make([]byte, 0))
	realSize, err := findBestSizeForH264Profile(h264SupportedProfile, size)
	fmt.Printf(realSize.String())
	if err != nil {
		return nil, err
	}
	opts := x264.Options{
		Width:     realSize.Width,
		Height:    realSize.Height,
		FrameRate: frameRate,
		Tune:      "zerolatency",
		Preset:    "veryfast",
		Profile:   "baseline",
		LogLevel:  x264.LogWarning,
	}
	encoder, err := x264.NewEncoder(buffer, &opts)
	if err != nil {
		return nil, err
	}
	return &H264Encoder{
		buffer:   buffer,
		encoder:  encoder,
		realSize: realSize,
	}, nil
}

// Encode encodes a frame into a h264 payload
func (e *H264Encoder) Encode(frame *image.RGBA) ([]byte, error) {
	err := e.encoder.Encode(frame)
	if err != nil {
		return nil, err
	}
	err = e.encoder.Flush()
	if err != nil {
		return nil, err
	}
	payload := e.buffer.Bytes()
	e.buffer.Reset()
	return payload, nil
}

// VideoSize returns the size the other side is expecting
func (e *H264Encoder) VideoSize() (size.Size, error) {
	return e.realSize, nil
}

// Close flushes and closes the inner x264 encoder
func (e *H264Encoder) Close() error {
	return e.encoder.Close()
}

// findBestSizeForH264Profile finds the best match given the size constraint and H264 profile
func findBestSizeForH264Profile(profile string, constraints size.Size) (size.Size, error) {
	profileSizes := map[string][]size.Size{
		"3.1": {
			{Width: 1920, Height: 1920},
			{Width: 1920, Height: 1440},
			{Width: 1920, Height: 1080},
			{Width: 1280, Height: 720},
			{Width: 720, Height: 576},
			{Width: 720, Height: 480},
			{Width: 320, Height: 240},
		},
	}
	if sizes, exists := profileSizes[profile]; exists {
		minRatioDiff := math.MaxFloat64
		var minRatioSize size.Size
		for _, size := range sizes {
			if size == constraints {
				return size, nil
			}
			lowerRes := size.Width <= constraints.Width && size.Height <= constraints.Height
			hRatio := float64(constraints.Width) / float64(size.Height)
			vRatio := float64(constraints.Width) / float64(size.Height)
			ratioDiff := math.Abs(hRatio - vRatio)
			if lowerRes && (ratioDiff) < 0.0001 {
				return size, nil
			} else if ratioDiff < minRatioDiff {
				minRatioDiff = ratioDiff
				minRatioSize = size
			}
		}
		fmt.Printf("RiatioSize %v", minRatioSize)
		return minRatioSize, nil
	}
	return size.Size{}, fmt.Errorf("Profile %s not supported", profile)
}

func init() {
	registeredEncoders[H264Codec] = newH264Encoder
}
