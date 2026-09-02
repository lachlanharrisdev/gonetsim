module github.com/lachlanharrisdev/gonetsim

go 1.26.1

require (
	github.com/emersion/go-sasl v0.0.0-20241020182733-b788ff22d5a6
	github.com/emersion/go-smtp v0.25.0
	github.com/fatih/color v1.19.0
	github.com/knadh/koanf/parsers/toml/v2 v2.2.2
	github.com/knadh/koanf/providers/confmap v1.0.1
	github.com/knadh/koanf/providers/file v1.2.1
	github.com/knadh/koanf/providers/structs v1.0.1
	github.com/knadh/koanf/v2 v2.3.6
	github.com/lmittmann/tint v1.2.0
	github.com/mattn/go-colorable v0.1.15
	github.com/mattn/go-isatty v0.0.24
	github.com/miekg/dns v1.1.73
	github.com/spf13/cobra v1.10.2
)

replace github.com/emersion/go-sasl => ./third_party/go-sasl

require (
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
