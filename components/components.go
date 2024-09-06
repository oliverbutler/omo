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
	return Footer(Class("prose-sm text-gray-500 mt-12"),
		P(
			g.Textf("© Oliver Butler "+strconv.Itoa(
				time.Now().Year())),
		),
	)
}

func HomePage() g.Node {
	return Header(
		Class("flex flex-row justify-between"),
		Div(Class("flex flex-col"),
			H2(
				Class("mb-2"),
				g.Text("Oliver Butler 🏔️"),
			),
			NavBar(),
		),
		Img(Src("/static/olly.webp"), Alt("Oliver Butler"), Class("rounded-full w-24 h-24")),
	)
}

func NavBar() g.Node {
	return Nav(Class("flex items-center gap-2"),
		A(Href("/"), g.Text("Home")),
		A(Href("/blog"), Class("no-underline"), g.Text("Blog")),
		A(Href("/hikes"), Class("no-underline"), g.Text("Hikes")),
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
			TitleEl(g.Text("Oliver Butler")),
			Link(Rel("stylesheet"), Href("/static/output.css")),
			g.Group(scripts),
		),
		Body(Class("mx-auto px-4 prose prose-invert"),
			body,
			PageFooter(),
		),
	)
}
