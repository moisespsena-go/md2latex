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
	"fmt"
	"os"
	"strings"
	"time"

	m2l "github.com/moisespsena-go/md2latex/pkg"
	"github.com/spf13/cobra"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "md2latex SRC DST",
	Short: "converts markdown to latex",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if len(args) == 0 {
			args = []string{viper.GetString("src"), viper.GetString("dst")}
		}
		if len(args) != 2 {
			return fmt.Errorf("accepts 2 arg(s), received %d", len(args))
		}

		var (
			flags = cmd.Flags()

			orSliceMap = func(a, b string) (ret []string) {
				if ret, _ = flags.GetStringSlice(a); len(ret) > 0 {
					return
				}
				if m := viper.GetStringMapString(b); len(m) > 0 {
					for k, v := range m {
						ret = append(ret, fmt.Sprintf("%s:%s", k, v))
					}
				}
				return nil
			}
			orString = func(a string) string {
				if v, _ := flags.GetString(a); len(v) > 0 {
					return v
				}
				return viper.GetString(a)
			}

			inputFile = args[0]
			config    = make(map[string]*m2l.LatexRaw)
			joined    = orString("joined")
			work      = orString("work-dir")

			opts = m2l.Opts{
				EnvQuotation: viper.GetString("latex.envs.quotation"),
			}
		)

		if work == "" {
			work = "."
		}

		if cfg := orSliceMap("latex-raw-file", "latex.raw-files"); len(cfg) > 0 {
			for _, v := range cfg {
				if pos := strings.IndexByte(v, ':'); pos > 0 {
					config[v[0:pos]] = &m2l.LatexRaw{Dst: v[pos+1:]}
				}
			}
		}

		return m2l.Exec(m2l.RunConfig{
			Input:         inputFile,
			JoinedOutput:  joined,
			RootDir:       work,
			Now:           time.Now(),
			LatexRawFiles: config,
			Output:        args[1],
			Opts:          opts,
		})
	},
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".md2latex.yaml", "config file (default is $HOME/.md2latex.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	flags := rootCmd.Flags()
	flags.StringSliceP("latex-raw-file", "R", []string{}, "latex raw files. Example: -R 'ID:DEST.tex'")
	flags.StringP("joined", "J", "", "name of joined markdown file. If not set, don't save it. Format: %D% (dir), %B% (base name without ext), %BE% (basename with ext)")
	flags.StringP("work-dir", "w", "", "work directory")
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
