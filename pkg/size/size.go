package size

import "fmt"

type Size struct {
	Width  int
	Height int
}

func (size *Size) String() string {
	return fmt.Sprintf("{Width: %v, Height %v}", size.Width, size.Height)
}
