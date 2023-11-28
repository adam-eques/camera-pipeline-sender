package encoders

import (
	"image"
	"io"

	"github.com/acentior/camera-pipeline-sender/pkg/size"
)

// Service creates encoder instances
type Service interface {
	NewEncoder(codec VideoCodec, size size.Size, frameRate int) (Encoder, error)
	Supports(codec VideoCodec) bool
}

// Encoder takes an image/frame and encodes it
type Encoder interface {
	io.Closer
	Encode(*image.RGBA) ([]byte, error)
	VideoSize() (size.Size, error)
}

// VideoCodec can be either h264 or vp8
type VideoCodec = int

const (
	// NoCodec "zero-value"
	NoCodec VideoCodec = iota
	// H264Codec h264
	H264Codec
	// VP8Codec vp8
	VP8Codec
)
