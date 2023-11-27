package signaling

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type SignalingSuit struct {
	suite.Suite
	sgl *Signaling
}

func (s *SignalingSuit) SetupSuite() {
	s.sgl = &Signaling{}
}

// run once, after test suite methods
func (s *SignalingSuit) TearDownSuite() {
}

func (s *SignalingSuit) SetupTest() {
}

// run after each test
func (s *SignalingSuit) TearDownTest() {
}

// run before each test
func (s *SignalingSuit) BeforeTest(cuiteName, testName string) {
}

// run after each test
func (s *SignalingSuit) AfterTest(cuiteName, testName string) {
}

// listen for 'go test' command --> run test methods
func TestSuite(t *testing.T) {
	suite.Run(t, new(SignalingSuit))
}

func (s *SignalingSuit) Test_Init() {
	err := s.sgl.Init("ws://127.0.0.1:8080")
	s.NoError(err)
	msg := NewWsMsg()
	s.sgl.SendMsg(msg)
}
