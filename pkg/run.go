package pkg

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

	bf "github.com/russross/blackfriday/v2"
)

type LatexRaw struct {
	Dst   string
	Value []string
}

type RunConfig struct {
	Now           time.Time
	Input         string
	Output        string
	RootDir       string
	JoinedOutput  string
	LatexRawFiles map[string]*LatexRaw
}

type DevNull struct {
}

func (DevNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (DevNull) Close() error {
	return nil
}

func Exec(cfg RunConfig) (err error) {
	var (
		input bytes.Buffer

		addFileToTarWriter = func(filePath string, data []byte, tarWriter *tar.Writer) (err error) {
			header := &tar.Header{
				Name:    filePath,
				Size:    int64(len(data)),
				Mode:    0666,
				ModTime: cfg.Now,
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
	)

	if cfg.LatexRawFiles == nil {
		cfg.LatexRawFiles = map[string]*LatexRaw{}
	}

	if cfg.RootDir == "" {
		cfg.RootDir = "."
	}

	if cfg.JoinedOutput != "" {
		if cfg.Input == "-" {
			cfg.JoinedOutput = ""
		} else {
			cfg.JoinedOutput = path.Clean(strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(
						cfg.JoinedOutput, "%D%", path.Dir(cfg.Input)),
					"%B%", strings.TrimSuffix(path.Base(cfg.Input), ".md")),
				"%BE%", path.Base(cfg.Input)))
		}
	}

	fmt.Fprintln(os.Stderr, "======>> begin", cfg.Input, "<<======")
	fmt.Fprintln(os.Stderr, "root dir: ", cfg.RootDir)
	fmt.Fprintln(os.Stderr, "joined output: ", cfg.JoinedOutput)
	defer fmt.Fprintln(os.Stderr, "======>> end", cfg.Input, "<<======")

	if err = ReadFile(&input, cfg.RootDir, path.Dir(cfg.Input), path.Base(cfg.Input)); err != nil {
		return
	}

	extensions := bf.CommonExtensions | bf.Titleblock
	renderer := &Renderer{Opts: Opts{
		HtmlBlockHandler: func(r *Renderer, w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
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
						if cfg, ok := cfg.LatexRawFiles[key]; ok {
							cfg.Value = append(cfg.Value, strings.TrimSpace(strings.TrimSuffix(s, "-->")))
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

	switch cfg.Output {
	case "-":
		os.Stdout.Write(result)
	default:
		if n := cfg.Output; strings.HasPrefix(n, "tar:") {
			n = n[4:]
			var main string
			var f io.Writer
			switch n[0] {
			case '-':
				f = os.Stdout
			default:
				parts := strings.Split(n, ":")
				switch len(parts) {
				case 1:
					n = parts[0]
				case 2:
					n = parts[0]
					main = parts[1]
				default:
					return fmt.Errorf("invalid DST value")
				}
				switch n {
				case "/dev/null":
					fmt.Println("@@ DEV NULL")
					f = DevNull{}
				default:
					fmt.Println("@@", n)
					var f2 *os.File
					if f2, err = os.Create(n); err != nil {
						return
					}
					f = f2
					defer f2.Close()
				}
			}
			if main == "" {
				main = cfg.Input[0:len(cfg.Input)-2] + "tex"
			}

			fmt.Println(fmt.Sprintf("++++ %T%[1]s", f))

			tarWriter := tar.NewWriter(f)
			defer tarWriter.Close()

			if cfg.JoinedOutput != "" {
				if err = addFileToTarWriter(cfg.JoinedOutput, input.Bytes(), tarWriter); err != nil {
					return
				}
			}

			if err = addFileToTarWriter(main, result, tarWriter); err != nil {
				return
			}
			var configNames []*LatexRaw

			for _, cfg := range cfg.LatexRawFiles {
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
			if cfg.JoinedOutput != "" {
				var f *os.File
				if f, err = os.Create(cfg.JoinedOutput); err != nil {
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
}
