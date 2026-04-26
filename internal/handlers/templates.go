package handlers

import (
	"html/template"
	"io/fs"
	"net/http"
)

type Templates struct {
	tmpl map[string]*template.Template
}

var pages = []struct {
	name   string
	layout string
}{
	{"public/index.html", "public/layout.html"},
	{"public/categories.html", "public/layout.html"},
	{"public/error.html", "public/layout.html"},
	{"admin/login.html", ""},
	{"admin/dashboard.html", "admin/layout.html"},
	{"admin/videos.html", "admin/layout.html"},
	{"admin/categories.html", "admin/layout.html"},
	{"admin/import.html", "admin/layout.html"},
}

func NewTemplates(fsys fs.FS, funcs template.FuncMap) (*Templates, error) {
	t := &Templates{tmpl: map[string]*template.Template{}}

	for _, p := range pages {
		var (
			tmpl *template.Template
			err  error
		)
		if p.layout != "" {
			tmpl, err = template.New("layout").Funcs(funcs).ParseFS(fsys, p.layout, p.name)
		} else {
			tmpl, err = template.New("layout").Funcs(funcs).ParseFS(fsys, p.name)
		}
		if err != nil {
			return nil, err
		}
		t.tmpl[p.name] = tmpl
	}

	return t, nil
}

func (t *Templates) Render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := t.tmpl[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (t *Templates) Error(w http.ResponseWriter, r *http.Request, code int, msg string) {
	w.WriteHeader(code)
	t.Render(w, "public/error.html", map[string]any{"Message": msg, "Code": code})
}
