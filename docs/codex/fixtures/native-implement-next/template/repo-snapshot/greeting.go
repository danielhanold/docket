package greeting

func Greet(name string) string {
	if name == "" {
		return "Hello!"
	}
	return "Hello, " + name + "!"
}
