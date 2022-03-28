/*
Copyright © 2022 Moises P. Sena <moisespsena@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unsafe"

	m2l "github.com/moisespsena-go/md2latex/pkg"
	bf "github.com/russross/blackfriday/v2"
	"github.com/spf13/cobra"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

type sConfig struct {
	Dst   string
	Value []string
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "md2latex SRC DST",
	Short: "converts markdown to latex",
	Args:  cobra.ExactArgs(2),
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			flags     = cmd.Flags()
			inputFile = args[0]
			config    = make(map[string]*sConfig)
			input     bytes.Buffer
			joined, _ = flags.GetString("joined")
		)

		if joined != "" {
			if inputFile == "-" {
				joined = ""
			} else {
				joined = path.Clean(strings.ReplaceAll(
					strings.ReplaceAll(
						strings.ReplaceAll(
							joined, "%D%", path.Dir(inputFile)),
						"%B%", strings.TrimSuffix(path.Base(inputFile), ".md")),
					"%BE%", path.Base(inputFile)))
			}
		}

		if cfg, _ := flags.GetStringSlice("latex-raw-file"); len(cfg) > 0 {
			for _, v := range cfg {
				if pos := strings.IndexByte(v, ':'); pos > 0 {
					config[v[0:pos]] = &sConfig{Dst: v[pos+1:]}
				}
			}
		}

		if err = m2l.ReadFile(&input, path.Dir(inputFile), path.Dir(inputFile), path.Base(inputFile)); err != nil {
			return
		}

		extensions := bf.CommonExtensions | bf.Titleblock
		renderer := &m2l.Renderer{Opts: m2l.Opts{
			HtmlBlockHandler: func(r *m2l.Renderer, w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
				switch node.Type {
				case bf.HTMLSpan:
					return bf.GoToNext
				case bf.HTMLBlock:
					p := unsafe.Pointer(&node.Literal)
					s := *(*string)(p)
					if strings.HasPrefix(s, "<!-- ::") {
						if pos := strings.Index(s, "\n"); pos > 0 {
							s := s[7:]
							key := s[:pos-7]
							if cfg, ok := config[key]; ok {
								cfg.Value = append(cfg.Value, strings.TrimSpace(strings.TrimSuffix(s, " -->")))
							}
						}
					}
					return bf.GoToNext
				}
				return bf.GoToNext
			},
		}}

		md := bf.New(bf.WithRenderer(renderer), bf.WithExtensions(extensions))

		ast := md.Parse(input.Bytes())
		result := renderer.Render(ast)

		switch args[1] {
		case "-":
			os.Stdout.Write(result)
		default:
			if n := args[1]; strings.HasPrefix(n, "tar:") {
				n = n[4:]
				var main string
				var f io.Writer
				switch n[0] {
				case '-':
					f = os.Stdout
				case ':':
					main = n[1:]
					n = main
					fallthrough
				default:
					var f *os.File
					if f, err = os.Create(n); err != nil {
						return
					}
					defer f.Close()
				}
				if main == "" {
					main = inputFile[0:len(inputFile)-2] + "tex"
				}
				tarWriter := tar.NewWriter(f)
				defer tarWriter.Close()

				if joined != "" {
					if err = addFileToTarWriter(joined, input.Bytes(), tarWriter); err != nil {
						return
					}
				}

				if err = addFileToTarWriter(main, result, tarWriter); err != nil {
					return
				}
				var configNames []*sConfig

				for _, cfg := range config {
					configNames = append(configNames, cfg)
				}
				sort.Slice(configNames, func(i, j int) bool {
					return configNames[i].Dst < configNames[j].Dst
				})
				for _, c := range configNames {
					if err = addFileToTarWriter(c.Dst, []byte(strings.Join(c.Value, "\n")), tarWriter); err != nil {
						return
					}
				}
			} else {
				if joined != "" {
					var f *os.File
					if f, err = os.Create(joined); err != nil {
						return
					}
					f.Write(input.Bytes())
					defer f.Close()
				}
				var f *os.File
				if f, err = os.Create(n); err != nil {
					return
				}
				f.Write(result)
				defer f.Close()
			}
		}

		return
	},
}

var now = time.Now()

func addFileToTarWriter(filePath string, data []byte, tarWriter *tar.Writer) (err error) {
	header := &tar.Header{
		Name:    filePath,
		Size:    int64(len(data)),
		Mode:    0666,
		ModTime: now,
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not write header for file '%s', got error '%s'", filePath, err.Error()))
	}

	_, err = tarWriter.Write(data)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error()))
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.md2latex.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	flags := rootCmd.Flags()
	flags.StringSliceP("latex-raw-file", "R", []string{}, "latex raw files. Example: -R 'ID:DEST.tex'")
	flags.StringP("joined", "J", "", "name of joined markdown file. If not set, don't save it. Format: %D% (dir), %B% (base name without ext), %BE% (basename with ext)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".md2latex" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".md2latex")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
