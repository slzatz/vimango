package vim

// Config holds vim configuration
type Config struct {
	UseGoImplementation bool
}

// DefaultConfig is the default configuration
var DefaultConfig = Config{
	UseGoImplementation: false,
}

// CurrentConfig is the current configuration
var CurrentConfig = DefaultConfig

// Configure sets up vim with the given configuration
func Configure(config Config) {
	CurrentConfig = config
	InitializeVim(config.UseGoImplementation, 0)
}