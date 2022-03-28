package pkg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func ReadFile(out io.Writer, root, dir, pth string) (err error) {
	var f *os.File
	if pth[0] == '/' {
		pth = path.Join(root, pth[1:])
	} else {
		pth = path.Join(dir, pth)
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
			if err = ReadFile(out, root, path.Dir(pth), npth); err != nil {
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
