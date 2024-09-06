package components

import (
	"os"
	"strconv"
	"time"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
)

const (
	ButtonStyle = "tw p-2 px-3 bg-gray-800/90 hover:bg-gray-700 rounded-md transition-all"
	InputStyle  = "tw border border-gray-800 px-2 rounded-md appearance-none focus:outline-none bg-transparent"
)

func PageFooter() g.Node {
	return Footer(Class("prose prose-sm text-gray-500 mt-12"),
		P(
			g.Textf("Copyright CrowdLog "+strconv.Itoa(
				time.Now().Year())+" - All rights reserved."),
		),
		P(
			g.Textf("Rendered %v. ", time.Now().Format(time.RFC3339)),
		),
	)
}

func DebugBody() g.Node {
	return Body(
		P(Class("text-red-500"), g.Text("DEBUG MODE 9")),
	)
}

func Page(body g.Node) g.Node {
	var scripts []g.Node
	scripts = append(scripts, Script(Src("https://unpkg.com/htmx.org@1.9.5/dist/htmx.min.js")))

	if os.Getenv("ENV") != "production" {
		scripts = append(scripts, Script(Src("/static/dev-reload.js")))
	}

	return HTML(
		Lang("en"),
		Head(
			TitleEl(g.Text("CrowdLog")),
			// Link(Rel("stylesheet"), Href("/assets/output.css")),
			g.Group(scripts),
		),
		Body(Class("mx-auto px-4 prose prose-invert"),
			body,
			PageFooter(),
		),
	)
}
