package pkg

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type FS interface {
	fs.FS
	fs.SubFS
	CreateAll(name string) (w io.WriteCloser, err error)
}

func containsAny(s, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

type DirFS string

func (dir DirFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) || runtime.GOOS == "windows" && containsAny(name, `\:`) {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrInvalid}
	}
	f, err := os.Open(string(dir) + "/" + name)
	if err != nil {
		return nil, err // nil fs.File
	}
	return f, nil
}

func (d DirFS) Sub(dir string) (f fs.FS, err error) {
	return DirFS(filepath.Join(string(d), dir)), err
}

func (d DirFS) CreateAll(name string) (w io.WriteCloser, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create %q: %s", name, err)
		}
	}()

	dp := path.Join(string(d), path.Dir(name))
	if err = os.MkdirAll(dp, 0775); err != nil {
		return
	}

	var f *os.File
	if f, err = os.Create(filepath.Join(string(d), name)); err != nil {
		return
	}
	return f, nil
}

type PathFS struct {
	FS      FS
	Dir     string
	RootDir string
}

func (c *PathFS) pathOf(name string) string {
	if name[0] == '/' {
		p := path.Join(c.RootDir, name)
		if rel, err := filepath.Rel(c.RootDir, p); err == nil && rel != "" {
			return rel
		}
		return p
	}
	return path.Join(c.Dir, name)
}

func (c PathFS) Sub(name string) (sub *PathFS, err error) {
	if name[0] == '/' {
		name = path.Clean(name)
		c.Dir = strings.TrimPrefix(name, "/")
	} else {
		c.Dir = path.Join(c.Dir, name)
	}
	return &c, nil
}

func (c *PathFS) ReadFile(out io.Writer, pth string) error {
	var count int
	return c.readFile(out, pth, &count, 0)
}

func (c *PathFS) CreateAll(name string) (w io.WriteCloser, err error) {
	return c.FS.CreateAll(c.pathOf(name))
}

func (c *PathFS) Open(name string) (fs.File, error) {
	return c.FS.Open(filepath.Join(c.Dir, name))
}

func (c *PathFS) readFile(out io.Writer, pth string, count *int, depth int) (err error) {
	(*count)++

	var f fs.File

	if depth == 0 {
		fmt.Fprintf(os.Stderr, "include %03d: %s: %s\n", *count, c.Dir, pth)
	} else {
		fmt.Fprintf(os.Stderr, "include %s %03d: %s: %s\n", strings.Repeat("--", depth), *count, c.Dir, pth)
	}

	if f, err = c.Open(pth); err != nil {
		return
	}

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	var (
		rline, prev string
		ln          int
	)

	for scanner.Scan() {
		ln++
		rline = scanner.Text()
		if line := strings.TrimSpace(rline); strings.HasPrefix(line, ":: ") && prev == "" {
			npth := strings.TrimSpace(line[2:])
			var sub *PathFS
			if sub, err = c.Sub(path.Dir(npth)); err != nil {
				return
			}
			if err = sub.readFile(out, path.Base(npth), count, depth+1); err != nil {
				return fmt.Errorf("from %s#%d: %s", pth, ln, err)
			}
			out.Write([]byte("\n"))
		} else {
			out.Write([]byte(rline))
			out.Write([]byte("\n"))
		}
		prev = rline
	}
	defer f.Close()
	return
}
