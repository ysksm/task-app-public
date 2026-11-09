package jira

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

var typeIcons = map[string][2]string{ // issue type id -> {color, letter}
	"10000": {"#8e4ec6", "E"},
	"10001": {"#3fae6a", "S"},
	"10002": {"#3e63dd", "F"},
	"10003": {"#4b9fd5", "T"},
	"10004": {"#d9a21b", "?"},
}

// avatar serves small SVG images for users, issue types and priorities.
func (h *handler) avatar(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	id := strings.TrimSuffix(chi.URLParam(r, "file"), ".svg")
	var svg string
	switch kind {
	case "user":
		u, err := h.svc.UserByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		color := u.AvatarColor
		if color == "" {
			color = "#6e6e80"
		}
		initial := "?"
		if rs := []rune(u.DisplayName); len(rs) > 0 {
			initial = strings.ToUpper(string(rs[0]))
		}
		svg = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48"><circle cx="24" cy="24" r="24" fill="%s"/>`+
			`<text x="24" y="31" font-family="sans-serif" font-size="22" font-weight="600" fill="#fff" text-anchor="middle">%s</text></svg>`,
			html.EscapeString(color), html.EscapeString(initial))
	case "issuetype":
		ic, ok := typeIcons[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		svg = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><rect width="16" height="16" rx="3" fill="%s"/>`+
			`<text x="8" y="12" font-family="sans-serif" font-size="10" font-weight="700" fill="#fff" text-anchor="middle">%s</text></svg>`, ic[0], ic[1])
	case "priority":
		var color string
		for _, p := range priorities {
			if p.ID == id {
				color = p.Color
			}
		}
		if color == "" {
			http.NotFound(w, r)
			return
		}
		svg = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M3 10l5-5 5 5" stroke="%s" stroke-width="2.5" fill="none" stroke-linecap="round"/></svg>`, color)
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "max-age=3600")
	_, _ = w.Write([]byte(svg))
}
