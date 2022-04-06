package pkg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func ReadFile(out io.Writer, root, dir, pth string) error {
	var count int
	return readFile(out, root, dir, pth, &count, 0)
}

func readFile(out io.Writer, root, dir, pth string, count *int, depth int) (err error) {
	(*count)++

	var f *os.File
	if pth[0] == '/' {
		pth = path.Join(root, pth[1:])
	} else {
		pth = path.Join(dir, pth)
	}

	if depth == 0 {
		fmt.Fprintf(os.Stderr, "include %03d: %s\n", *count, pth)
	} else {
		fmt.Fprintf(os.Stderr, "include %03d: %s %s\n", *count, strings.Repeat("--", depth), pth)
	}

	if f, err = os.Open(pth); err != nil {
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
			npth := strings.TrimSpace(line[3:])
			if err = readFile(out, root, path.Dir(pth), npth, count, depth+1); err != nil {
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

func FormatFileName(fmt, name string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(
				fmt, "%D%", path.Dir(name)),
			"%B%", strings.TrimSuffix(path.Base(name), ".md")),
		"%BE%", path.Base(name))
}
