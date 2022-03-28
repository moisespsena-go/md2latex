package pkg_test

import (
	"fmt"

	bflatex "github.com/moisespsena-go/md2latex/pkg"
	bf "github.com/russross/blackfriday/v2"
)

func Example() {
	const input = `<!-- data
created: 2022-03-25T14:44:40-03:00
modified: 2022-03-25T14:44:50-03:00
type: Checklist
-->

# Section

Some _Markdown_ text.

## Subsection

Foobar.
`

	extensions := bf.CommonExtensions | bf.Titleblock
	renderer := &bflatex.Renderer{Opts: bflatex.Opts{
		Author:    "John Doe",
		Languages: "english,french",
		Flags:     bflatex.TOC,
	}}
	md := bf.New(bf.WithRenderer(renderer), bf.WithExtensions(extensions))

	ast := md.Parse([]byte(input))
	fmt.Printf("%s\n", renderer.Render(ast))
	// Output:
	// \section{Section}
	// Some \emph{Markdown} text.
	//
	// \subsection{Subsection}
	// Foobar.
}
