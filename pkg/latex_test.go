package pkg

import (
	"testing"

	bf "github.com/russross/blackfriday/v2"
)

const input = `
% Title

# Section

Some _Markdown_ text. Some **strong** text.

Some escaped charaters: &%#

Some "quoted" text.

## Subsection

[Link](http://www.example.com)

# Other section

* Item 1: _code_

		Embedded "code"

* Item 2: footnote[^foo].

Enumeration:

1. Enum 1
2. Enum 2

Definition list
: This is a Markdown extension

[^foo]: bar

> Quote
> over
> several lines

Inline code: ` + "`fmt.Println(s)`" + `

Some strikethrough: ~~deleted~~.

Image: ![Image 1](1.jpg)
Linked image: ![Image 2](https://www.example.com/2.jpg)

A table:

| Header 1  | Header 2  |
|-----------|-----------|
| Content 1 | Content 2 |

Horizontal rule:

---

End
`

type testData struct {
	input string
	want  string
	flags Flag
	ext   bf.Extensions
}

func runTest(t *testing.T, tdt []testData) {
	for _, v := range tdt {
		renderer := &Renderer{Opts: Opts{Flags: v.flags}}
		md := bf.New(bf.WithRenderer(renderer), bf.WithExtensions(v.ext))
		ast := md.Parse([]byte(v.input))
		got := string(renderer.Render(ast))
		if v.want != got {
			t.Errorf("got %q, want %q", got, v.want)
		}
	}
}

func TestCodeInline(t *testing.T) {
	tdt := []testData{
		{input: "`foo`", want: `\lstinline!foo!` + "\n"},
		{input: "`foo!`", want: `\lstinline"foo!"` + "\n"},
		{input: "`foo!\"#$%&'()`", want: `\lstinline+foo!"#$%&'()+` + "\n"},
	}

	runTest(t, tdt)
}

func TestCodeBlock(t *testing.T) {
	tdt := []testData{
		{
			input: `	foo`,
			want: `\begin{lstlisting}[language=]
foo
\end{lstlisting}

`},
		{
			input: "``` go" + `
foo
` + "```",
			want: `\begin{lstlisting}[language=go]
foo
\end{lstlisting}

`,
			ext: bf.FencedCode},
	}

	runTest(t, tdt)
}

func TestEmph(t *testing.T) {
	tdt := []testData{
		{input: `_foo_`, want: `\emph{foo}` + "\n"},
		{input: `_foo_bar_`, want: `\emph{foo}bar\_` + "\n"},
		// TODO: Upstream bug?
		// {input: `_foo_bar_`, want: `\emph{foo\_bar}` + "\n", ext: bf.NoIntraEmphasis},
		// {input: `*foo*bar*`, want: `\emph{foo*bar}` + "\n", ext: bf.NoIntraEmphasis},
		{input: `*foo_bar*`, want: `\emph{foo\_bar}` + "\n"},
		{input: `*foo_bar*`, want: `\emph{foo\_bar}` + "\n", ext: bf.NoIntraEmphasis},
		{input: `**foo**`, want: `\textbf{foo}` + "\n"},
	}

	runTest(t, tdt)
}

func TestEscape(t *testing.T) {
	tdt := []testData{
		{input: `abcd#$%~_{}&`, want: `abcd\#\$\%\~\_\{\}\&` + "\n"},
		{input: `a\a\ `, want: `a\textbackslash{}a` + "\n"},
		{input: `a\#\$\%\~\_\{\}\&`, want: `a\#\textbackslash{}\$\textbackslash{}\%\~\_\{\}\&` + "\n"},
	}

	runTest(t, tdt)
}

func TestFootnote(t *testing.T) {
	tdt := []testData{
		{
			input: `[^foo]
[^foo]: bar`,
			want: `\href{bar}{^foo}` + "\n",
		},
		{
			input: `[^foo]
[^foo]: bar`,
			want: `\footnote{bar}` + "\n\n",
			ext:  bf.Footnotes,
		},
	}

	runTest(t, tdt)
}

func TestHardbreak(t *testing.T) {
	tdt := []testData{
		{
			input: `foo
bar`, want: `foo~\\
bar
`,
			ext: bf.HardLineBreak},
	}

	runTest(t, tdt)
}

func TestHRule(t *testing.T) {
	tdt := []testData{
		{input: `---`, want: `\HRule{}` + "\n"},
	}

	runTest(t, tdt)
}

func TestImage(t *testing.T) {
	tdt := []testData{
		{
			input: `![Image 1](foobar.jpg)`,
			want: `\begin{center}
\includegraphics[max width=\textwidth, max height=\textheight]{foobar}
\end{center}

`,
		},
		{
			input: `![Image 1](http://example.com/foobar.jpg)`,
			want:  `\url{http://example.com/foobar.jpg}` + "\n",
		},
		{
			input: `![Image 1](foobar.jpg "foo")`,
			want: `\begin{figure}[!ht]
\begin{center}
\includegraphics[max width=\textwidth, max height=\textheight]{foobar}
\end{center}
\caption{foo}
\end{figure}

`,
		},
	}

	runTest(t, tdt)
}

func TestLink(t *testing.T) {
	tdt := []testData{
		{input: `[foo](http://example.com)`, want: `\href{http://example.com}{foo}` + "\n"},
		{input: `[foo](mailto://doe@example.com)`, want: `\href{mailto://doe@example.com}{foo}` + "\n"},
		{
			input: `http://example.com`,
			want:  `\href{http://example.com}{http://example.com}` + "\n",
			ext:   bf.Autolink,
		},
		{
			input: `<mailto://doe@example.com>`,
			want:  `\href{mailto://doe@example.com}{doe@example.com}` + "\n",
			ext:   bf.Autolink,
		},
		{
			input: `<mailto:doe@example.com>`,
			want:  `\href{mailto:doe@example.com}{doe@example.com}` + "\n",
			ext:   bf.Autolink,
		},
		{
			input: `<doe@example.com>`,
			want:  `\href{mailto:doe@example.com}{doe@example.com}` + "\n",
			ext:   bf.Autolink,
		},
		{
			input: `[foo](http://example.com)`,
			want:  `foo\footnote{\nolinkurl{http://example.com}}` + "\n",
			flags: SkipLinks,
		},
		{
			input: `http://example.com`,
			want:  `\nolinkurl{http://example.com}` + "\n",
			ext:   bf.Autolink,
			flags: SkipLinks,
		},
	}

	runTest(t, tdt)
}

func TestList(t *testing.T) {
	tdt := []testData{
		{
			input: `* foo
* bar`,
			want: `\begin{itemize}
\item foo
\item bar
\end{itemize}

`},
		{
			input: `1. foo
2. bar`,
			want: `\begin{enumerate}
\item foo
\item bar
\end{enumerate}

`},
		{
			input: `foo
: bar

baz
: qux`,
			want: `\begin{description}
\item [foo] bar
\item [baz] qux
\end{description}

`, ext: bf.DefinitionLists},
		{
			input: `foo
: bar

baz
: qux`,
			want: `foo
: bar

baz
: qux
`},
	}

	runTest(t, tdt)
}

func TestQuotation(t *testing.T) {
	tdt := []testData{
		{
			input: `> Quote`,
			want: `\begin{quotation}
Quote
\end{quotation}

`},
	}

	runTest(t, tdt)
}

func TestQuote(t *testing.T) {
	tdt := []testData{
		{input: `"foo"`, want: `\enquote{foo}` + "\n"},
	}

	runTest(t, tdt)
}

func TestSection(t *testing.T) {
	tdt := []testData{
		{input: `#foo`, want: `\section{foo}` + "\n"},
		{input: `# foo`, want: `\section{foo}` + "\n"},
		{input: `## foo`, want: `\subsection{foo}` + "\n"},
		{input: `### foo`, want: `\subsubsection{foo}` + "\n"},
		{input: `#### foo`, want: `\paragraph{foo} `},
		{input: `##### foo`, want: `\subparagraph{foo} `},
		{input: `###### foo`, want: `\textbf{foo} `},
	}

	runTest(t, tdt)
}

func TestStrikethrough(t *testing.T) {
	tdt := []testData{
		{input: `~~foo~~`, want: `\~\~foo\~\~` + "\n"},
		{input: `~~foo~~`, want: `\sout{foo}` + "\n", ext: bf.Strikethrough},
	}

	runTest(t, tdt)
}

func TestTable(t *testing.T) {
	tdt := []testData{
		{
			input: `
| default | left | center | right |
|---------|:-----|:------:|------:|
| foo     | bar  | baz    | qux   |
`,
			want: `\begin{center}
\begin{tabular}{llcr}
\textbf{default} & \textbf{left} & \textbf{center} & \textbf{right} \\
\hline
foo & bar & baz & qux \\
\end{tabular}
\end{center}

`,
			ext: bf.Tables,
		},
		{
			input: `
| default |
|---------|
| foo     |
`,
			want: `| default |
|---------|
| foo     |
`,
		},
	}

	runTest(t, tdt)
}

func TestTitleblock(t *testing.T) {
	tdt := []testData{
		{
			input: `% Title
% Continuing title
Normal text`,
			want: `Normal text
`,
			ext: bf.Titleblock,
		},
		{
			input: `% Title
% Continuing title
Normal text`,
			want: `\% Title
\% Continuing title
Normal text
`,
		},
	}

	runTest(t, tdt)
}

/*
func TestDummy(t *testing.T) {
	extensions := bf.CommonExtensions | bf.TOC | bf.Titleblock
	extensions |= bf.Footnotes
	flags := CompletePage

	ast := bf.Parse([]byte(input), bf.Options{Extensions: extensions})
	renderer := Renderer{
		Author:     "John Doe",
		Languages:  "english,french",
		Extensions: extensions,
		Flags:      flags,
	}

	fmt.Printf("%s\n", renderer.Render(ast))
}
*/

func BenchmarkRender(b *testing.B) {
	extensions := bf.CommonExtensions | bf.Titleblock
	extensions |= bf.Footnotes
	flags := CompletePage | TOC

	renderer := &Renderer{Opts: Opts{
		Author:    "John Doe",
		Languages: "english,french",
		Flags:     flags,
	}}

	md := bf.New(bf.WithExtensions(extensions), bf.WithRenderer(renderer))
	ast := md.Parse([]byte(input))

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		renderer.Render(ast)
	}
}
