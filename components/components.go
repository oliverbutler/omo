package components

import (
	"os"
	"strconv"
	"time"

	g "github.com/maragudk/gomponents"
	hx "github.com/maragudk/gomponents-htmx"
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

type NavItem struct {
	Text string
	Href string
}

func NavBar(selectedPath string) g.Node {
	navItems := []NavItem{
		{Text: "Home", Href: "/"},
		{Text: "Hikes", Href: "/hikes"},
	}

	navItemNodes := []g.Node{}

	for _, item := range navItems {
		if item.Href == selectedPath {
			navItemNodes = append(navItemNodes, A(Class(""), g.Text(item.Text)))
		} else {
			navItemNodes = append(navItemNodes, A(Class("no-underline"), Href(item.Href), g.Text(item.Text)))
		}
	}

	return Header(
		Class("flex flex-row justify-between max-w-4xl mx-auto"),
		Div(Class("flex flex-col"),
			H2(
				Class("mb-2"),
				g.Text("Oliver Butler 🏔️"),
			),
			Nav(Class("max-w-4xl flex items-center gap-2"),
				g.Group(navItemNodes),
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
		Script(Src("/static/olly.js")),
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
			hx.Boost("swap"),
			body,
		),
	)
}
