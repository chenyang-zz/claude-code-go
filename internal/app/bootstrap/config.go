package bootstrap

type Config struct {
	AppName string
}

func DefaultConfig() Config {
	return Config{AppName: "claude-code-go"}
}
