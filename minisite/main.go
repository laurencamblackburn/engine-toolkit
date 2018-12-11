package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gobuffalo/plush"
	"github.com/russross/blackfriday"
)

func main() {
	http.Handle("/experiments/engine-toolkit", http.RedirectHandler("/veritone/engine-toolkit", http.StatusMovedPermanently))
	http.HandleFunc("/veritone/engine-toolkit", handleIndex())
	http.Handle("/veritone/engine-toolkit/static/", http.StripPrefix("/veritone/engine-toolkit/static/", http.FileServer(http.Dir("static"))))
	addr := "0.0.0.0:" + os.Getenv("PORT")
	log.Println("listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalln(err)
	}
}

func handleIndex() http.HandlerFunc {
	var err error
	b, err := ioutil.ReadFile("index.template.html")
	if err != nil {
		return errorHandler(err, http.StatusInternalServerError)
	}
	tpl, err := plush.Parse(string(b))
	if err != nil {
		return errorHandler(err, http.StatusInternalServerError)
	}
	docsb, err := ioutil.ReadFile("docs.md")
	if err != nil {
		return errorHandler(err, http.StatusInternalServerError)
	}
	docs := string(renderMarkdown(docsb))
	return func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ctx := plush.NewContextWithContext(r.Context())
		ctx.Set("content", template.HTML(docs))
		out, err := tpl.Exec(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := io.WriteString(w, out); err != nil {
			log.Println("ERR: write response:", err)
		}
	}
}

func errorHandler(err error, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err.Error(), status)
	}
}

var blackfridayExtensions = blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
	blackfriday.EXTENSION_TABLES |
	blackfriday.EXTENSION_FENCED_CODE |
	blackfriday.EXTENSION_AUTOLINK |
	blackfriday.EXTENSION_STRIKETHROUGH |
	blackfriday.EXTENSION_SPACE_HEADERS |
	blackfriday.EXTENSION_HEADER_IDS |
	blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
	blackfriday.EXTENSION_DEFINITION_LISTS |
	blackfriday.EXTENSION_AUTO_HEADER_IDS

var blackfridayHTMLOptions = blackfriday.HTML_USE_XHTML |
	blackfriday.HTML_USE_SMARTYPANTS |
	blackfriday.HTML_SMARTYPANTS_FRACTIONS |
	blackfriday.HTML_SMARTYPANTS_DASHES |
	blackfriday.HTML_SMARTYPANTS_LATEX_DASHES |
	blackfriday.HTML_TOC

func renderMarkdown(src []byte) []byte {
	//src = append([]byte("<br>\n"), src...)
	markdownRenderer := blackfriday.HtmlRenderer(blackfridayHTMLOptions, "", "")
	return blackfriday.MarkdownOptions(src, markdownRenderer, blackfriday.Options{Extensions: blackfridayExtensions})
}
