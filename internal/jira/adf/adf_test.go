package adf

import (
	"encoding/json"
	"strings"
	"testing"
)

func toJSON(t *testing.T, n *Node) string {
	t.Helper()
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFromMarkdownStructure(t *testing.T) {
	got := toJSON(t, FromMarkdown("## 背景\n\n**太字** と *斜体* と `code` と [リンク](https://example.com)\n\n- a\n- b\n\n1. x\n2. y\n\n```go\nfmt.Println(1)\n```\n\n> quote\n\n---"))
	for _, want := range []string{
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"背景"}]}`,
		`{"type":"text","text":"太字","marks":[{"type":"strong"}]}`,
		`{"type":"text","text":"斜体","marks":[{"type":"em"}]}`,
		`{"type":"text","text":"code","marks":[{"type":"code"}]}`,
		`{"type":"text","text":"リンク","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}`,
		`{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}`,
		`{"type":"orderedList","attrs":{"order":1}`,
		`{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println(1)"}]}`,
		`{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quote"}]}]}`,
		`{"type":"rule"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s\nin %s", want, got)
		}
	}
	if !strings.HasPrefix(got, `{"type":"doc","version":1,"content":[`) {
		t.Errorf("not a doc: %s", got)
	}
}

func TestFromMarkdownEmpty(t *testing.T) {
	if FromMarkdown("  \n") != nil {
		t.Fatal("empty markdown should give nil")
	}
}

func TestLineBreaksBecomeHardBreaks(t *testing.T) {
	got := toJSON(t, FromMarkdown("一行目\n二行目"))
	want := `{"type":"paragraph","content":[{"type":"text","text":"一行目"},{"type":"hardBreak"},{"type":"text","text":"二行目"}]}`
	if !strings.Contains(got, want) {
		t.Fatalf("got %s", got)
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []string{
		"plain text",
		"## 見出し\n\n本文 **強調** *斜体* `x` ~~消~~ [l](https://e.com)",
		"- a\n- b\n  - nested",
		"1. one\n2. two",
		"```js\nconst a = 1\n```",
		"> quoted",
		"a\n\n---\n\nb",
	}
	for _, md := range cases {
		doc := FromMarkdown(md)
		raw, _ := json.Marshal(doc)
		back, err := ToMarkdown(raw)
		if err != nil {
			t.Fatal(err)
		}
		if back != md {
			t.Errorf("round trip changed\n in: %q\nout: %q", md, back)
		}
	}
}

func TestToMarkdownFromOrcaStyleDoc(t *testing.T) {
	// ORCA's textToAdf: one paragraph per line, empty lines as empty paragraphs.
	raw := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"line 1"}]},{"type":"paragraph","content":[]},{"type":"paragraph","content":[{"type":"text","text":"line 2"}]}]}`
	got, err := ToMarkdown(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got != "line 1\n\nline 2" {
		t.Fatalf("got %q", got)
	}
}

func TestToMarkdownLenient(t *testing.T) {
	for raw, want := range map[string]string{
		`null`:           "",
		`"plain string"`: "plain string",
		`{"type":"doc","version":1,"content":[{"type":"panel","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"text":"@alice"}},{"type":"text","text":" hi"}]}]}]}`: "@alice hi",
	} {
		got, err := ToMarkdown(json.RawMessage(raw))
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v", raw, got, err)
		}
	}
	if _, err := ToMarkdown(json.RawMessage(`{"type":"paragraph"}`)); err == nil {
		t.Error("expected an error for a non-doc root")
	}
}
