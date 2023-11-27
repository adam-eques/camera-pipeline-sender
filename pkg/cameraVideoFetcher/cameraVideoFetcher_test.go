package cameraVideoFetcher

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type SenderSuit struct {
	suite.Suite
	sender *CameraVideoFetcher
}

func (s *SenderSuit) SetupSuite() {
	s.sender = &CameraVideoFetcher{}
}

// run once, after test suite methods
func (s *SenderSuit) TearDownSuite() {
}

func (s *SenderSuit) SetupTest() {
}

// run after each test
func (s *SenderSuit) TearDownTest() {
}

// run before each test
func (s *SenderSuit) BeforeTest(cuiteName, testName string) {
}

// run after each test
func (s *SenderSuit) AfterTest(cuiteName, testName string) {
}

// listen for 'go test' command --> run test methods
func TestSuite(t *testing.T) {
	suite.Run(t, new(SenderSuit))
}

func (s *SenderSuit) Test_CameraConnect() {
	err := s.sender.ConnectCamera()

	s.NoError(err)
}

func (s *SenderSuit) Test_IsCameraConnected() {
	flag := s.sender.IsCameraConnected()

	s.True(flag, "Camera is not connected")
}
