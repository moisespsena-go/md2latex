// Copyright © 2013-2016 Pierre Neidhardt <ambrevar@gmail.com>
// Use of this file is governed by the license that can be found in LICENSE.

// Package latex is a LaTeX renderer for the Blackfriday Markdown processor.
package pkg

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"unsafe"

	bf "github.com/russross/blackfriday/v2"
	"github.com/shopspring/decimal"
)

type Opts struct {

	// Flags allow customizing this renderer's behavior.
	Flags Flag

	// The document author displayed by the `\maketitle` command.
	// This will only display if the `Titleblock` extension is on and a title is
	// present.
	Author string

	// The languages to be used by the `babel` package.
	// Languages must be comma-spearated.
	Languages string

	EnvQuotation string

	Titled bool

	HtmlBlockHandler func(r *Renderer, w io.Writer, node *bf.Node, entering bool) bf.WalkStatus
}

var WriteString = io.WriteString

func WriteByte(w io.Writer, b byte) (n int, err error) {
	return w.Write([]byte{b})
}

func WriteRune(w io.Writer, r rune) (n int, err error) {
	if r < utf8.RuneSelf {
		return w.Write([]byte{byte(r)})
	}
	return w.Write([]byte(string(r)))
}

// Renderer is a type that implements the Renderer interface for LaTeX
// output.
type Renderer struct {
	Opts

	// If text is within quotes.
	quoted    bool
	quoteOpen bool
}

func NewRenderer(opts Opts) *Renderer {
	if opts.EnvQuotation == "" {
		opts.EnvQuotation = "quotation"
	}
	return &Renderer{Opts: opts}
}

// Flag controls the options of the renderer.
type Flag int

const (
	FlagsNone Flag = 0

	// CompletePage generates a complete LaTeX document, preamble included.
	CompletePage Flag = 1 << iota

	// ChapterTitle uses the titleblock (if the extension is on) as chapter title.
	// Ignored when CompletePage is on.
	ChapterTitle

	// No paragraph indentation.
	NoParIndent

	SkipLinks // Never link.
	Safelink  // Only link to trusted protocols.

	TOC // Generate the table of content.
)

var cellAlignment = [4]byte{
	0:                       'l',
	bf.TableAlignmentLeft:   'l',
	bf.TableAlignmentRight:  'r',
	bf.TableAlignmentCenter: 'c',
}

var latexEscaper = map[rune][]byte{
	'#':  []byte(`\#`),
	'$':  []byte(`\$`),
	'%':  []byte(`\%`),
	'&':  []byte(`\&`),
	'\\': []byte(`\textbackslash{}`),
	'_':  []byte(`\_`),
	'{':  []byte(`\{`),
	'}':  []byte(`\}`),
	'~':  []byte(`\~`),
	'\'': []byte(``),
}

var headers = []string{
	`chapter`,
	`section`,
	`subsection`,
	`subsubsection`,
	`paragraph`,
	`subparagraph`,
}

func (r *Renderer) Escape(w io.Writer, t []byte) {
	text := []rune(string(t))
	for i := 0; i < len(text); i++ {
		// directly copy normal characters
		org := i

		for i < len(text) && latexEscaper[text[i]] == nil {
			i++
		}

		if i > org {
			w.Write([]byte(string(text[org:i])))
			if i >= len(text) {
				break
			}
		}

		// escape a character
		switch text[i] {
		case '"':
			if r.quoted {
				WriteRune(w, '“')
				r.quoted = false
			} else {
				WriteRune(w, '“')
				r.quoted = true
			}
		case '\'':
			if r.quoted {
				if r.quoteOpen && i < len(text) {
					switch text[i+1] {
					case '\r', '\n', ' ', '\t', '.':
						WriteRune(w, '’')
					}
				} else {
					WriteRune(w, '‘')
				}
				r.quoted = false
				r.quoteOpen = false
			} else {
				if i > 0 {
					switch text[i-1] {
					case '\r', '\n', ' ', '\t', '.':
						WriteRune(w, '‘')
						r.quoted = true
						r.quoteOpen = true
					default:
						WriteRune(w, '’')
					}
				} else {
					WriteRune(w, '‘')
					r.quoted = true
					r.quoteOpen = true
				}
			}
		default:
			w.Write(latexEscaper[text[i]])
		}
	}
}

func languageAttr(info []byte) []byte {
	if len(info) == 0 {
		return nil
	}
	endOfLang := bytes.IndexAny(info, "\t ")
	if endOfLang < 0 {
		return info
	}
	return info[:endOfLang]
}

func (r *Renderer) Env(w io.Writer, environment string, entering bool, args ...string) {
	if entering {
		WriteString(w, `\begin{`+environment+"}")
		for _, arg := range args {
			WriteString(w, fmt.Sprintf("{%s}", arg))
		}
		WriteString(w, "\n")
	} else {
		WriteString(w, `\end{`+environment+"}\n\n")
	}
}

func (r *Renderer) Cmd(w io.Writer, command string, entering bool) {
	if entering {
		WriteString(w, `\`+command+`{`)
	} else {
		WriteByte(w, '}')
	}
}

// Return the first ASCII character that is not in 'text'.
// The resulting delimiter cannot be '*' nor space.
func getDelimiter(text []byte) byte {
	delimiters := make([]bool, 256)
	for _, v := range text {
		delimiters[v] = true
	}
	// '!' is the character after space in the ASCII encoding.
	for k := byte('!'); k < byte('*'); k++ {
		if !delimiters[k] {
			return k
		}
	}
	// '+' is the character after '*' in the ASCII encoding.
	for k := byte('+'); k < 128; k++ {
		if !delimiters[k] {
			return k
		}
	}
	return 0
}

func hasPrefixCaseInsensitive(s, prefix []byte) bool {
	if len(s) < len(prefix) {
		return false
	}
	delta := byte('a' - 'A')
	for i, b := range prefix {
		if b != s[i] && b != s[i]+delta {
			return false
		}
	}
	return true
}

// RenderNode renders a single node.
// As a rule of thumb to enforce consistency, each node is responsible for
// appending the needed line breaks. Line breaks are never prepended.
func (r *Renderer) RenderNode(w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
	switch node.Type {
	case bf.BlockQuote:
		var args []string
		if entering && node.LastChild.Type == bf.Paragraph && node.LastChild.LastChild.Type == bf.Text {
			// detech format:
			// > teste
			// > -- Author
			text := node.LastChild.LastChild
			if len(text.Literal) > 0 {
				p := unsafe.Pointer(&text.Literal)
				s := *(*string)(p)
				if pos := strings.LastIndexByte(s, '\n'); pos > 0 {
					if lastLine := s[pos+1:]; strings.HasPrefix(lastLine, "-- ") {
						args = append(args, strings.TrimSpace(lastLine[3:]))
						text.Literal = text.Literal[0:pos]
					}
				} else if pos == -1 && strings.HasPrefix(s, "-- ") {
					args = append(args, strings.TrimSpace(s[3:]))
					if text.Prev != nil {
						text.Prev.Next = nil
						text.Parent.LastChild = text.Prev
					} else if node.LastChild.Prev != nil {
						node.LastChild = node.LastChild.Prev
						node.LastChild.Next = nil
					}
				}
			}
		}
		r.Env(w, r.EnvQuotation, entering, args...)

	case bf.Code:
		// TODO: Reach a consensus for math syntax.
		if bytes.HasPrefix(node.Literal, []byte("$$ ")) {
			// Inline math
			WriteByte(w, '$')
			w.Write(node.Literal[3:])
			WriteByte(w, '$')
			break
		}
		// 'lstinline' needs an ASCII delimiter that is not in the node content.
		// TODO: Find a more elegant fallback for when the code lists all ASCII characters.
		delimiter := getDelimiter(node.Literal)
		WriteString(w, `\lstinline`)
		if delimiter != 0 {
			WriteByte(w, delimiter)
			w.Write(node.Literal)
			WriteByte(w, delimiter)
		} else {
			WriteString(w, "!<RENDERING ERROR: no delimiter found>!")
		}

	case bf.CodeBlock:
		lang := languageAttr(node.Info)
		if bytes.Compare(lang, []byte("math")) == 0 {
			WriteString(w, "\\[\n")
			w.Write(node.Literal)
			WriteString(w, "\\]\n\n")
			break
		}
		WriteString(w, `\begin{lstlisting}[language=`)
		w.Write(lang)
		WriteString(w, "]\n")
		w.Write(node.Literal)
		WriteString(w, `\end{lstlisting}`+"\n\n")

	case bf.Del:
		r.Cmd(w, "sout", entering)

	case bf.Document:
		break

	case bf.Emph:
		r.Cmd(w, "emph", entering)

	case bf.Hardbreak:
		WriteString(w, `~\\`+"\n")

	case bf.Heading:
		if node.IsTitleblock {
			// Nothing to print but its children.
			break
		}
		if entering {
			if n := node.Level - 1; n < len(headers) {
				WriteByte(w, '\\')
				WriteString(w, headers[n])
				if len(node.HeadingData.Config) > 0 {
					cfg := string(node.HeadingData.Config[:])
					switch cfg {
					case "*", "**":
						if node.FirstChild != nil && node.FirstChild.Type == bf.Text {
							WriteString(w, "*[")
							w.Write(node.FirstChild.Literal)
							if cfg == "*" {
								WriteString(w, "]{")
								w.Write(node.FirstChild.Literal)
								WriteString(w, "}\n\\addcontentsline{toc}{")
								WriteString(w, headers[n])
								WriteByte(w, '}')
							} else {
								WriteByte(w, ']')
							}
						}
					}
				}
				WriteByte(w, '{')
			} else {
				WriteString(w, `\textbf{`)
			}
		} else {
			WriteByte(w, '}')
			switch node.Level {
			// Paragraph need no newline.
			case 1, 2, 3:
				WriteByte(w, '\n')
			default:
				WriteByte(w, ' ')
			}
		}

	case bf.HTMLBlock:
		if r.HtmlBlockHandler != nil {
			return r.HtmlBlockHandler(r, w, node, entering)
		}
		// HTML code makes no sense in LaTeX.
		break

	case bf.HTMLSpan:
		if r.HtmlBlockHandler != nil {
			return r.HtmlBlockHandler(r, w, node, entering)
		}
		// HTML code makes no sense in LaTeX.
		break

	case bf.HorizontalRule:
		WriteString(w, `\HRule{}`+"\n")

	case bf.Image:
		if entering {
			dest := node.LinkData.Destination
			if hasPrefixCaseInsensitive(dest, []byte("http://")) || hasPrefixCaseInsensitive(dest, []byte("https://")) {
				WriteString(w, `\url{`)
				w.Write(dest)
				WriteByte(w, '}')
				return bf.SkipChildren
			}
			if node.LinkData.Title != nil {
				WriteString(w, `\begin{figure}[!ht]`+"\n")
			}
			WriteString(w, `\begin{center}`+"\n")
			// Trim extension so that LaTeX loads the most appropriate file.
			ext := filepath.Ext(string(dest))
			dest = dest[:len(dest)-len(ext)]
			WriteString(w, `\includegraphics[max width=\textwidth, max height=\textheight]{`)
			w.Write(dest)
			WriteString(w, "}\n"+`\end{center}`+"\n")
			if node.LinkData.Title != nil {
				WriteString(w, `\caption{`)
				w.Write(node.LinkData.Title)
				WriteString(w, "}\n"+`\end{figure}`+"\n")
			}
		}
		return bf.SkipChildren

	case bf.Item:
		if entering {
			if node.ListFlags&bf.ListTypeTerm != 0 {
				WriteString(w, `\item [`)
			} else if node.ListFlags&bf.ListTypeDefinition == 0 {
				WriteString(w, `\item `)
			}
		} else {
			if node.ListFlags&bf.ListTypeTerm != 0 {
				WriteString(w, "] ")
			}
		}

	case bf.Link:
		// TODO: Relative links do not make sense in LaTeX. Print a warning?
		dest := node.LinkData.Destination

		// Raw URI
		if needSkipLink(r.Flags, dest) {
			if node.FirstChild != node.LastChild || node.FirstChild.Type != bf.Text || bytes.Compare(dest, node.FirstChild.Literal) != 0 {
				if !entering {
					WriteString(w, `\footnote{\nolinkurl{`)
					w.Write(dest)
					WriteString(w, `}}`)
				}
				break
			}
			// Link content (only one Text child) and destination are identical (e.g.
			// with autolink).
			WriteString(w, `\nolinkurl{`)
			w.Write(dest)
			WriteByte(w, '}')
			return bf.SkipChildren
		}

		// Footnotes
		if node.NoteID != 0 {
			if entering {
				WriteString(w, `\footnote{`)
				w := &bytes.Buffer{}
				footnoteNode := node.LinkData.Footnote
				footnoteNode.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
					if node == footnoteNode {
						return bf.GoToNext
					}
					return r.RenderNode(w, node, entering)
				})
				w.Write(w.Bytes())
				WriteString(w, `}`)
			}
			break
		}

		// Normal link
		if entering {
			WriteString(w, `\href{`)
			w.Write(dest)
			WriteString(w, `}{`)
		} else {
			WriteByte(w, '}')
		}

	case bf.List:
		if node.IsFootnotesList {
			// The footnote list is not needed for LaTeX as the footnotes are rendered
			// directly from the links.
			return bf.SkipChildren
		}
		listType := "itemize"
		if node.ListFlags&bf.ListTypeOrdered != 0 {
			listType = "enumerate"
		}
		if node.ListFlags&bf.ListTypeDefinition != 0 {
			listType = "description"
		}
		r.Env(w, listType, entering)

	case bf.Paragraph:
		if !entering {
			// If paragraph is the term of a definition list, don't insert new lines.
			if node.Parent.Type != bf.Item || node.Parent.ListFlags&bf.ListTypeTerm == 0 {
				WriteByte(w, '\n')
				// Don't insert an additional linebreak after last node of an item, a quote, etc.
				if node.Next != nil {
					WriteByte(w, '\n')
				}
			}
		}

	case bf.Softbreak:
		// TODO: Upstream does not use it. If status changes, linebreaking should be
		// updated.
		break

	case bf.Strong:
		r.Cmd(w, "textbf", entering)

	case bf.Table:
		border := node.TableData.Border

		if entering {
			WriteString(w, `\begin{center}`+"\n"+`\begin{tabular}{`)
			node.Walk(func(c *bf.Node, entering bool) bf.WalkStatus {
				if c.Type == bf.TableCell && entering {
					i := 0

					var writed, sep bool
					if border.Left {
						WriteByte(w, '|')
					}
					for cell := c; cell != nil; cell = cell.Next {
						writed = false
						sep = border.Column

						if cell.TableCellData.Opts == nil {
							if cell.TableCellData.IsLast {
								sep = false
							}
						} else {
							var width decimal.Decimal
							if v, ok := cell.TableCellData.Opts["width"]; ok {
								width, _ = v.(decimal.Decimal)
							}
							if sep {
								if cell.TableCellData.IsLast {
									sep = false
								}
							} else if v, ok := cell.TableCellData.Opts["sep"]; ok {
								sep, _ = v.(bool)
							}
							if !width.IsZero() {
								WriteString(w, fmt.Sprintf(`p{%s\textwidth}`, width))
								switch cell.Align {
								case bf.TableAlignmentRight:
									WriteString(w, `<{\raggedleft\arraybackslash}`)
								case bf.TableAlignmentCenter:
									WriteString(w, `<{\centering\arraybackslash}`)
								case bf.TableAlignmentLeft:
								}
								writed = true
							}
						}
						if !writed {
							WriteByte(w, cellAlignment[cell.Align])
						}
						if sep {
							WriteByte(w, '|')
						}
						i++
					}

					if !sep && border.Rigth {
						WriteByte(w, '|')
					}
					return bf.Terminate
				}
				return bf.GoToNext
			})
			WriteString(w, "}\n")
			if border.Top {
				WriteString(w, "\\hline\n")
			}
		} else {
			if border.Bottom {
				WriteString(w, "\\hline\n")
			}
			WriteString(w, `\end{tabular}`+"\n"+`\end{center}`+"\n\n")
		}

	case bf.TableBody:
		// Nothing to do here.
		break

	case bf.TableCell:
		if node.IsHeader {
			r.Cmd(w, "textbf", entering)
		}
		if !entering && node.Next != nil {
			WriteString(w, " & ")
		}

	case bf.TableHead:
		if !entering {
			WriteString(w, `\hline`+"\n")
		}

	case bf.TableRow:
		if !entering {
			if node.Parent.Parent.TableData.Border.Row {
				if node.Parent.Type == bf.TableBody {
					if node.TableRowData.IsLast {
						WriteString(w, ` \\`+"\n")
					} else {
						WriteString(w, ` \\ \hline`+"\n")
					}
				} else {
					WriteString(w, ` \\`+"\n")
				}
			} else {
				WriteString(w, ` \\`+"\n")
			}
		}

	case bf.Text:
		if len(node.Literal) > 0 {
			r.Escape(w, node.Literal)
		}
		break

	default:
		panic("Unknown node type " + node.Type.String())
	}
	return bf.GoToNext
}

// Get title: concatenate all Text children of Titleblock.
func getTitle(ast *bf.Node) []byte {
	titleRenderer := Renderer{}
	var buf bytes.Buffer

	ast.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if node.Type == bf.Heading && node.HeadingData.IsTitleblock && entering {
			node.Walk(func(c *bf.Node, entering bool) bf.WalkStatus {
				return titleRenderer.RenderNode(&buf, c, entering)
			})
			return bf.Terminate
		}
		return bf.GoToNext
	})
	return buf.Bytes()
}

func hasFigures(ast *bf.Node) bool {
	result := false
	ast.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if node.Type == bf.Image && node.LinkData.Title != nil {
			result = true
			return bf.Terminate
		}
		return bf.GoToNext
	})
	return result
}

// RenderHeader prints the LaTeX preamble if CompletePage is on.
func (r *Renderer) RenderHeader(w io.Writer, ast *bf.Node) {
	var title string

	if r.Flags&CompletePage != 0 {
		title = string(getTitle(ast))

		// TODO: Color source code and links?
		io.WriteString(w, `\documentclass{article}

\usepackage[utf8]{inputenc}
\usepackage[T1]{fontenc}
\usepackage{lmodern}
\usepackage{marvosym}
\usepackage{textcomp}
\DeclareUnicodeCharacter{20AC}{\EUR{}}
\DeclareUnicodeCharacter{2260}{\neq}
\DeclareUnicodeCharacter{2264}{\leq}
\DeclareUnicodeCharacter{2265}{\geq}
\DeclareUnicodeCharacter{22C5}{\cdot}
\DeclareUnicodeCharacter{A0}{~}
\DeclareUnicodeCharacter{B1}{\pm}
\DeclareUnicodeCharacter{D7}{\times}

\usepackage{amsmath}
\usepackage[export]{adjustbox} % loads also graphicx
\usepackage{listings}
\usepackage[margin=1in]{geometry}
\usepackage{verbatim}
\usepackage[normalem]{ulem}
\usepackage{hyperref}

\lstset{
	numbers=left,
	breaklines=true,
	xleftmargin=2\baselineskip,
	showstringspaces=false,
	basicstyle=\ttfamily,
	keywordstyle=\bfseries\color{green!40!black},
	commentstyle=\itshape\color{purple!40!black},
	stringstyle=\color{orange},
	numberstyle=\ttfamily,
	literate=
	{á}{{\'a}}1 {é}{{\'e}}1 {í}{{\'i}}1 {ó}{{\'o}}1 {ú}{{\'u}}1
	{Á}{{\'A}}1 {É}{{\'E}}1 {Í}{{\'I}}1 {Ó}{{\'O}}1 {Ú}{{\'U}}1
	`)
		io.WriteString(w,
			"{à}{{\\`a}}1 {è}{{\\`e}}1 {ì}{{\\`i}}1 {ò}{{\\`o}}1 {ù}{{\\`u}}1"+
				"\n\t"+
				"{À}{{\\`A}}1 {È}{{\\'E}}1 {Ì}{{\\`I}}1 {Ò}{{\\`O}}1 {Ù}{{\\`U}}1")
		io.WriteString(w, `
	{ä}{{\"a}}1 {ë}{{\"e}}1 {ï}{{\"i}}1 {ö}{{\"o}}1 {ü}{{\"u}}1
	{Ä}{{\"A}}1 {Ë}{{\"E}}1 {Ï}{{\"I}}1 {Ö}{{\"O}}1 {Ü}{{\"U}}1
	{â}{{\^a}}1 {ê}{{\^e}}1 {î}{{\^i}}1 {ô}{{\^o}}1 {û}{{\^u}}1
	{Â}{{\^A}}1 {Ê}{{\^E}}1 {Î}{{\^I}}1 {Ô}{{\^O}}1 {Û}{{\^U}}1
	{œ}{{\oe}}1 {Œ}{{\OE}}1 {æ}{{\ae}}1 {Æ}{{\AE}}1 {ß}{{\ss}}1
	{ű}{{\H{u}}}1 {Ű}{{\H{U}}}1 {ő}{{\H{o}}}1 {Ő}{{\H{O}}}1
	{ç}{{\c c}}1 {Ç}{{\c C}}1 {ø}{{\o}}1 {å}{{\r a}}1 {Å}{{\r A}}1
	{€}{{\EUR}}1 {£}{{\pounds}}1
}
`)

		if r.Languages != "" {
			io.WriteString(w, "\n"+`\usepackage[`+r.Languages+`]{babel}`+"\n")
		}

		io.WriteString(w, `\usepackage{csquotes}

\hypersetup{colorlinks,
	citecolor=black,
	filecolor=black,
	linkcolor=black,
	linktoc=page,
	urlcolor=black,
	pdfstartview=FitH,
	breaklinks=true,
	pdfauthor={Blackfriday Markdown Processor v`)
		io.WriteString(w, bf.Version)
		io.WriteString(w, `},
}

\newcommand{\HRule}{\rule{\linewidth}{0.5mm}}
\addtolength{\parskip}{0.5\baselineskip}
`)

		if r.Flags&NoParIndent != 0 {
			io.WriteString(w, `\parindent=0pt
`)
		}

		if title != "" {
			io.WriteString(w, `
\title{`+title+`}
\author{`+r.Author+`}
`)
		}

		io.WriteString(w, `
\begin{document}
`)

		if title != "" {
			WriteString(w, `
\maketitle
`)
			if r.Flags&TOC != 0 {
				WriteString(w, `\vfill
\thispagestyle{empty}

\tableofcontents
`)
				if hasFigures(ast) {
					io.WriteString(w, `\listoffigures
`)
				}
				io.WriteString(w, `\clearpage
`)
			}
		}

		io.WriteString(w, "\n\n")
	} else if r.Flags&ChapterTitle != 0 && strings.TrimSpace(title) != "" {
		io.WriteString(w, `\chapter{`+title+"}\n\n")
	}
}

// RenderHeader prints the '\end{document}' if CompletePage is on.
func (r *Renderer) RenderFooter(w io.Writer, ast *bf.Node) {
	if r.Flags&CompletePage != 0 {
		io.WriteString(w, `\end{document}`+"\n")
	}
}

// Render prints out the whole document from the ast, header and footer included.
func (r *Renderer) Render(w io.Writer, ast *bf.Node) {
	r.RenderHeader(w, ast)
	ast.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if node.Type == bf.Heading && node.HeadingData.IsTitleblock {
			return bf.SkipChildren
		}
		return r.RenderNode(w, node, entering)
	})

	r.RenderFooter(w, ast)
}

// Run prints out the whole document with CompletePage and TOC flags enabled.
func Run(w io.Writer, input []byte, opts ...bf.Option) {
	renderer := &Renderer{Opts: Opts{Flags: CompletePage | TOC}}

	optList := []bf.Option{bf.WithRenderer(renderer), bf.WithExtensions(bf.CommonExtensions)}
	optList = append(optList, opts...)
	parser := bf.New(optList...)
	ast := parser.Parse(input)
	renderer.Render(w, ast)
}
