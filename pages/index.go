package pages

import (
	"oliverbutler/components"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
)

func Index() g.Node {
	return components.Page(Div(
		components.NavBar(),
		Div(Class("max-w-4xl mx-auto"),
			P(g.Text("Home content"))),
	))
}
