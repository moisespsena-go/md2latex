module github.com/moisespsena-go/md2latex

go 1.16

require (
	github.com/mitchellh/go-homedir v1.1.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/shopspring/decimal v1.3.1
	github.com/spf13/cobra v1.4.0
	github.com/spf13/viper v1.10.1
)

// local dev: replace github.com/russross/blackfriday/v2 => ../../../github.com/russross/blackfriday

// prod:
// o mod edit -replace github.com/russross/blackfriday/v2=github.com/moisespsena-go/blackfriday/v2@v2.1.0-fenced-table && go mod tidy
replace github.com/russross/blackfriday/v2 => github.com/moisespsena-go/blackfriday/v2 v2.1.1-0.20220406181319-5ad01bb6f259
