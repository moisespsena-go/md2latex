package pkg

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
	Opts          Opts
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

		createFile = func(pth string, data []byte) (err error) {
			pth = filepath.Join(cfg.RootDir, pth)
			defer func() {
				if err != nil {
					err = fmt.Errorf("create %q: %s", pth, err)
				}
			}()
			d := filepath.Dir(pth)
			if err = os.MkdirAll(d, 0775); err != nil {
				return
			}
			var f *os.File
			if f, err = os.Create(pth); err != nil {
				return
			}
			defer f.Close()
			_, err = f.Write(data)
			return
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
			cfg.JoinedOutput = path.Clean(FormatFileName(cfg.JoinedOutput, cfg.Input))
		}
	}

	fmt.Fprintln(os.Stderr, "======>> begin", cfg.Input, "<<======")
	fmt.Fprintln(os.Stderr, "root dir: ", cfg.RootDir)
	fmt.Fprintln(os.Stderr, "joined output: ", cfg.JoinedOutput)
	defer fmt.Fprintln(os.Stderr, "======>> end", cfg.Input, "<<======")

	if err = ReadFile(&input, cfg.RootDir, filepath.Join(cfg.RootDir, path.Dir(cfg.Input)), path.Base(cfg.Input)); err != nil {
		return
	}

	cfg.Opts.HtmlBlockHandler = func(r *Renderer, w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
		switch node.Type {
		case bf.HTMLSpan:
			return bf.GoToNext
		case bf.HTMLBlock:
			p := unsafe.Pointer(&node.Literal)
			s := *(*string)(p)
			if strings.HasPrefix(s, "<!-- ::") {
				if pos := strings.Index(s, "\n"); pos > 0 {
					key := s[7:pos]
					if key == "" {
						// raw latex code
						s = strings.TrimSpace(strings.TrimSuffix(s[pos+1:], "-->"))
						w.Write([]byte(s))
						w.Write([]byte("\n\n"))
					} else if cfg, ok := cfg.LatexRawFiles[key]; ok {
						cfg.Value = append(cfg.Value, strings.TrimSpace(strings.TrimSuffix(s[pos+1:], "-->")))
					}
				}
			}
			return bf.GoToNext
		}
		return bf.GoToNext
	}

	extensions := bf.CommonExtensions | bf.Footnotes | bf.DefinitionLists
	renderer := NewRenderer(cfg.Opts)

	md := bf.New(
		bf.WithFileName(cfg.Input),
		bf.WithRootDir(cfg.RootDir),
		bf.WithRenderer(renderer),
		bf.WithExtensions(extensions),
	)

	ast := md.Parse(input.Bytes())

	var (
		result bytes.Buffer
		w      io.Writer = &result
	)
	if cfg.Output == "-" {
		w = os.Stdout
	}

	renderer.Render(w, ast)

	var configNames []*LatexRaw

	for _, cfg := range cfg.LatexRawFiles {
		configNames = append(configNames, cfg)
	}
	sort.Slice(configNames, func(i, j int) bool {
		return configNames[i].Dst < configNames[j].Dst
	})

	switch cfg.Output {
	case "-":
	default:
		if n := cfg.Output; strings.HasPrefix(n, "tar:") {
			n = n[4:]
			var main string
			var f io.Writer
			parts := strings.Split(n, ":")
			switch parts[0] {
			case "-":
				f = os.Stdout
			default:
				n = parts[0]
				switch n {
				case "/dev/null":
					f = DevNull{}
				default:
					var f2 *os.File
					if f2, err = os.Create(n); err != nil {
						return
					}
					f = f2
					defer f2.Close()
				}
			}
			switch len(parts) {
			case 1:
			case 2:
				main = parts[1]
			default:
				return fmt.Errorf("invalid DST value")
			}
			if main == "" {
				main = cfg.Input[0:len(cfg.Input)-2] + "tex"
			}

			tarWriter := tar.NewWriter(f)
			defer tarWriter.Close()

			if cfg.JoinedOutput != "" {
				if err = addFileToTarWriter(cfg.JoinedOutput, input.Bytes(), tarWriter); err != nil {
					return
				}
			}

			if err = addFileToTarWriter(main, result.Bytes(), tarWriter); err != nil {
				return
			}

			for _, c := range configNames {
				if err = addFileToTarWriter(c.Dst, []byte(strings.Join(c.Value, "\n")), tarWriter); err != nil {
					return
				}
			}
		} else {
			if cfg.JoinedOutput != "" {
				if err = createFile(cfg.JoinedOutput, input.Bytes()); err != nil {
					return
				}
			}
			if err = createFile(n, result.Bytes()); err != nil {
				return
			}
			for _, c := range configNames {
				if err = createFile(c.Dst, []byte(strings.Join(c.Value, "\n"))); err != nil {
					return
				}
			}
		}
	}

	return
}
