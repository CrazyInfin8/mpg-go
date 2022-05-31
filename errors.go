package mpg

import (
	"fmt"
)

type ExpectedHeader struct{}

func (err ExpectedHeader) Error() string { return "Unable to find header" }

type CouldNotOpenFile struct {
	path string
}

func (err CouldNotOpenFile) Error() string{ return fmt.Sprintf("Could not open file: %q", err.path) }
