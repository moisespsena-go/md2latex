module github.com/moisespsena-go/md2latex

go 1.16

require (
	github.com/mitchellh/go-homedir v1.1.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/spf13/cobra v1.4.0
	github.com/spf13/viper v1.10.1
)

replace github.com/russross/blackfriday/v2 => github.com/moisespsena-go/blackfriday/v2 v2.1.1-0.20220329171430-f976c2179d2a
