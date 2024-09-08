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

func NavBar() g.Node {
	return Header(
		Class("flex flex-row justify-between max-w-4xl mx-auto"),
		Div(Class("flex flex-col"),
			H2(
				Class("mb-2"),
				g.Text("Oliver Butler 🏔️"),
			),
			Nav(Class("max-w-4xl flex items-center gap-2"),
				A(Href("/"), g.Text("Home")),
				A(Href("/blog"), Class("no-underline"), g.Text("Blog")),
				A(Href("/hikes"), Class("no-underline"), g.Text("Hikes")),
			),
		),
		Img(Src("/static/olly.webp"), Alt("Oliver Butler"), Class("rounded-full w-24 h-24")),
	)
}

func Page(body g.Node, extraHead ...g.Node) g.Node {
	headContent := []g.Node{
		TitleEl(g.Text("Oliver Butler")),
		Link(Rel("stylesheet"), Href("/static/output.css")),
		Script(Src("https://unpkg.com/htmx.org@1.9.5/dist/htmx.min.js")),
	}

	if os.Getenv("ENV") != "production" {
		headContent = append(headContent, Script(Src("/static/dev-reload.js")))
	}

	headContent = append(headContent, extraHead...)

	return HTML(
		Class("prose prose-invert max-w-none"),
		Lang("en"),
		Head(g.Group(headContent)),
		Body(
			body,
		),
	)
}
