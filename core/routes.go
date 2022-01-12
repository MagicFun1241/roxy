package core

import "fmt"

func FormatWildcard(path string) string {
	if path == "/" {
		return "/*"
	} else {
		return fmt.Sprintf("%s/*", path)
	}
}
