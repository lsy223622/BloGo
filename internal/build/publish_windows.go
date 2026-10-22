package build

import "fmt"

func publicationSupported() error                { return fmt.Errorf("production publication requires Linux") }
func atomicCurrent(string, string, string) error { return publicationSupported() }
