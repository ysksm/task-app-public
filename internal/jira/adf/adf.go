// Package adf converts between Markdown (how descriptions and comments are
// stored) and the Atlassian Document Format used by Jira REST API v3.
//
// Supported nodes: doc, paragraph, heading, bulletList, orderedList,
// listItem, codeBlock, blockquote, rule, hardBreak and text with the marks
// strong, em, code, strike and link. Other nodes are reduced to their text.
package adf

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Node is an ADF node.
type Node struct {
	Type    string         `json:"type"`
	Version int            `json:"version,omitempty"` // doc only
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []*Node        `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// Mark is a text decoration.
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Doc returns an empty document.
func Doc(content ...*Node) *Node {
	if content == nil {
		content = []*Node{}
	}
	return &Node{Type: "doc", Version: 1, Content: content}
}

// ---- Markdown -> ADF ----

var md = goldmark.New(goldmark.WithExtensions(extension.Strikethrough))

// FromMarkdown converts Markdown to an ADF document. Empty input gives nil so
// callers can return `null` like Jira does for an empty description.
func FromMarkdown(src string) *Node {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	source := []byte(src)
	root := md.Parser().Parse(text.NewReader(source))
	c := converter{src: source}
	return Doc(c.blocks(root)...)
}

type converter struct{ src []byte }

func (c converter) blocks(parent ast.Node) []*Node {
	out := []*Node{}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		if b := c.block(n); b != nil {
			out = append(out, b)
		}
	}
	return out
}

func (c converter) block(n ast.Node) *Node {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return &Node{Type: "paragraph", Content: c.inlines(n, nil)}
	case *ast.Heading:
		return &Node{Type: "heading", Attrs: map[string]any{"level": n.Level}, Content: c.inlines(n, nil)}
	case *ast.List:
		if n.IsOrdered() {
			return &Node{Type: "orderedList", Attrs: map[string]any{"order": n.Start}, Content: c.blocks(n)}
		}
		return &Node{Type: "bulletList", Content: c.blocks(n)}
	case *ast.ListItem:
		content := c.blocks(n)
		if len(content) == 0 {
			content = []*Node{{Type: "paragraph"}}
		}
		return &Node{Type: "listItem", Content: content}
	case *ast.FencedCodeBlock:
		node := &Node{Type: "codeBlock", Content: textContent(c.lines(n))}
		if lang := string(n.Language(c.src)); lang != "" {
			node.Attrs = map[string]any{"language": lang}
		}
		return node
	case *ast.CodeBlock:
		return &Node{Type: "codeBlock", Content: textContent(c.lines(n))}
	case *ast.Blockquote:
		return &Node{Type: "blockquote", Content: c.blocks(n)}
	case *ast.ThematicBreak:
		return &Node{Type: "rule"}
	case *ast.HTMLBlock:
		return &Node{Type: "paragraph", Content: textContent(strings.TrimRight(c.lines(n), "\n"))}
	}
	if n.HasChildren() {
		return &Node{Type: "paragraph", Content: c.inlines(n, nil)}
	}
	return nil
}

func (c converter) lines(n ast.Node) string {
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		b.Write(seg.Value(c.src))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func textContent(s string) []*Node {
	if s == "" {
		return nil
	}
	return []*Node{{Type: "text", Text: s}}
}

func (c converter) inlines(parent ast.Node, marks []Mark) []*Node {
	out := []*Node{}
	add := func(s string) {
		if s == "" {
			return
		}
		// Merge with the previous text node when the marks are identical.
		if k := len(out) - 1; k >= 0 && out[k].Type == "text" && sameMarks(out[k].Marks, marks) {
			out[k].Text += s
			return
		}
		out = append(out, &Node{Type: "text", Text: s, Marks: copyMarks(marks)})
	}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			add(string(n.Segment.Value(c.src)))
			if n.HardLineBreak() || n.SoftLineBreak() {
				out = append(out, &Node{Type: "hardBreak"})
			}
		case *ast.String:
			add(string(n.Value))
		case *ast.CodeSpan:
			var b strings.Builder
			for t := n.FirstChild(); t != nil; t = t.NextSibling() {
				if s, ok := t.(*ast.Text); ok {
					b.Write(s.Segment.Value(c.src))
				}
			}
			out = append(out, &Node{Type: "text", Text: b.String(), Marks: append(copyMarks(marks), Mark{Type: "code"})})
		case *ast.Emphasis:
			typ := "em"
			if n.Level >= 2 {
				typ = "strong"
			}
			out = append(out, c.inlines(n, append(copyMarks(marks), Mark{Type: typ}))...)
		case *east.Strikethrough:
			out = append(out, c.inlines(n, append(copyMarks(marks), Mark{Type: "strike"}))...)
		case *ast.Link:
			link := Mark{Type: "link", Attrs: map[string]any{"href": string(n.Destination)}}
			out = append(out, c.inlines(n, append(copyMarks(marks), link))...)
		case *ast.AutoLink:
			url := string(n.URL(c.src))
			href := url
			if n.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(href, "mailto:") {
				href = "mailto:" + href
			}
			link := Mark{Type: "link", Attrs: map[string]any{"href": href}}
			out = append(out, &Node{Type: "text", Text: url, Marks: append(copyMarks(marks), link)})
		case *ast.Image:
			alt := plainText(n, c.src)
			if alt == "" {
				alt = string(n.Destination)
			}
			link := Mark{Type: "link", Attrs: map[string]any{"href": string(n.Destination)}}
			out = append(out, &Node{Type: "text", Text: alt, Marks: append(copyMarks(marks), link)})
		case *ast.RawHTML:
			for i := 0; i < n.Segments.Len(); i++ {
				seg := n.Segments.At(i)
				add(string(seg.Value(c.src)))
			}
		default:
			out = append(out, c.inlines(n, marks)...)
		}
	}
	// A trailing break carries no meaning in ADF.
	for len(out) > 0 && out[len(out)-1].Type == "hardBreak" {
		out = out[:len(out)-1]
	}
	return out
}

func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			b.Write(t.Segment.Value(src))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func copyMarks(m []Mark) []Mark {
	if len(m) == 0 {
		return nil
	}
	return append([]Mark(nil), m...)
}

func sameMarks(a, b []Mark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || fmt.Sprint(a[i].Attrs) != fmt.Sprint(b[i].Attrs) {
			return false
		}
	}
	return true
}

// ---- ADF -> Markdown ----

// ToMarkdown converts an ADF document to Markdown. It also accepts a plain
// JSON string (some clients send v2-style text) and null.
func ToMarkdown(raw json.RawMessage) (string, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "", nil
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return "", err
		}
		return str, nil
	}
	var n Node
	if err := json.Unmarshal(raw, &n); err != nil {
		return "", fmt.Errorf("not a valid ADF document: %w", err)
	}
	if n.Type != "doc" {
		return "", fmt.Errorf("not a valid ADF document: the root node must be \"doc\"")
	}
	return NodeToMarkdown(&n), nil
}

// NodeToMarkdown converts a parsed ADF tree to Markdown.
func NodeToMarkdown(n *Node) string {
	out := renderBlock(n)
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}

func renderBlocks(nodes []*Node, sep string) string {
	parts := []string{}
	for _, c := range nodes {
		if s := renderBlock(c); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep)
}

func renderBlock(n *Node) string {
	switch n.Type {
	case "doc":
		return renderBlocks(n.Content, "\n\n")
	case "paragraph":
		return renderInlines(n.Content)
	case "heading":
		level := intAttr(n.Attrs, "level", 1)
		if level < 1 || level > 6 {
			level = 1
		}
		return strings.Repeat("#", level) + " " + strings.TrimSpace(renderInlines(n.Content))
	case "bulletList", "orderedList":
		start := intAttr(n.Attrs, "order", 1)
		items := []string{}
		for i, item := range n.Content {
			prefix := "- "
			if n.Type == "orderedList" {
				prefix = fmt.Sprintf("%d. ", start+i)
			}
			items = append(items, renderListItem(item, prefix))
		}
		return strings.Join(items, "\n")
	case "listItem":
		return renderListItem(n, "- ")
	case "codeBlock":
		lang, _ := n.Attrs["language"].(string)
		return "```" + lang + "\n" + strings.TrimSuffix(plainInlines(n.Content), "\n") + "\n```"
	case "blockquote":
		lines := strings.Split(renderBlocks(n.Content, "\n\n"), "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight("> "+l, " ")
		}
		return strings.Join(lines, "\n")
	case "rule":
		return "---"
	case "text", "hardBreak", "mention", "emoji", "inlineCard", "date", "status":
		return renderInlines([]*Node{n})
	case "table":
		rows := []string{}
		for _, row := range n.Content {
			cells := []string{}
			for _, cell := range row.Content {
				cells = append(cells, strings.ReplaceAll(renderBlocks(cell.Content, " "), "\n", " "))
			}
			rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
		}
		return strings.Join(rows, "\n")
	}
	// panel, expand, mediaSingle, ... : keep whatever text they contain.
	if len(n.Content) > 0 {
		if isInline(n.Content[0]) {
			return renderInlines(n.Content)
		}
		return renderBlocks(n.Content, "\n\n")
	}
	return n.Text
}

func renderListItem(item *Node, prefix string) string {
	body := renderBlocks(item.Content, "\n")
	indent := strings.Repeat(" ", len(prefix))
	lines := strings.Split(body, "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = prefix + lines[i]
		} else if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " ")
}

func isInline(n *Node) bool {
	switch n.Type {
	case "text", "hardBreak", "mention", "emoji", "inlineCard", "date", "status", "mediaInline":
		return true
	}
	return false
}

func plainInlines(nodes []*Node) string {
	var b strings.Builder
	for _, n := range nodes {
		if n.Type == "hardBreak" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(n.Text)
	}
	return b.String()
}

func renderInlines(nodes []*Node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			b.WriteString(applyMarks(n.Text, n.Marks))
		case "hardBreak":
			b.WriteString("  \n")
		case "mention":
			b.WriteString(strAttr(n.Attrs, "text"))
		case "emoji":
			if t := strAttr(n.Attrs, "text"); t != "" {
				b.WriteString(t)
			} else {
				b.WriteString(strAttr(n.Attrs, "shortName"))
			}
		case "inlineCard":
			b.WriteString(strAttr(n.Attrs, "url"))
		case "status":
			b.WriteString(strAttr(n.Attrs, "text"))
		default:
			b.WriteString(renderInlines(n.Content))
		}
	}
	return b.String()
}

func applyMarks(s string, marks []Mark) string {
	if s == "" {
		return ""
	}
	var href string
	for _, m := range marks {
		switch m.Type {
		case "code":
			s = "`" + s + "`"
		case "strong":
			s = "**" + s + "**"
		case "em":
			s = "*" + s + "*"
		case "strike":
			s = "~~" + s + "~~"
		case "link":
			href = strAttr(m.Attrs, "href")
		}
	}
	if href != "" {
		s = "[" + s + "](" + href + ")"
	}
	return s
}

func strAttr(a map[string]any, k string) string {
	s, _ := a[k].(string)
	return s
}

func intAttr(a map[string]any, k string, def int) int {
	switch v := a[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}
