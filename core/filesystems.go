package core

import "fmt"

const (
	LocationDefault = iota
	LocationGeneral
)

func FormatFilesystemHandlerKey(location, server, route int) string {
	return fmt.Sprintf("%d_%d_%d", location, server, route)
}
