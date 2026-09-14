package greeting

import "strings"

func Greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Hello!"
	}
	return "Hello, " + name + "!"
}
