package components

import (
	"context"
	"oliverbutler/lib"
	"oliverbutler/lib/logging"
	"oliverbutler/lib/users"
	"os"
	"strconv"
	"time"

	g "github.com/maragudk/gomponents"
	hx "github.com/maragudk/gomponents-htmx"
	. "github.com/maragudk/gomponents/html"
)

const (
	ButtonStyle = "p-2 px-3 bg-gray-900/70 hover:bg-gray-800 rounded-md transition-all"
)

func PageFooter(ctx context.Context, app *lib.App) g.Node {
	visits, err := app.Users.IncrementVisitorCount(ctx)
	if err != nil {
		logging.OmoLogger.ErrorContext(ctx, "Error incrementing visitor count", err)
		visits = 0
	}

	return Footer(Class("flex max-w-4xl mx-auto text-gray-500 mt-12"),
		P(
			g.Textf("© Oliver Butler "+strconv.Itoa(
				time.Now().Year())),
			g.Textf(" Generated at %s", time.Now().Format("2006-01-02 15:04:05")),
			g.Textf(" | %d visits", visits),
		),
	)
}

type NavItem struct {
	Text string
	Href string
}

func NavBar(selectedPath string, app *lib.App, user *users.UserContext) g.Node {
	navItems := []NavItem{
		{Text: "Home", Href: "/"},
		{Text: "Photos", Href: "/photos"},
		{Text: "Hikes", Href: "/hikes"},
	}

	navItemNodes := []g.Node{}

	for _, item := range navItems {
		if item.Href == selectedPath {
			navItemNodes = append(navItemNodes, A(Class(""), Href(item.Href), g.Text(item.Text)))
		} else {
			navItemNodes = append(navItemNodes, A(Class("no-underline"), Href(item.Href), g.Text(item.Text)))
		}
	}

	return Header(
		Class("flex flex-row justify-between max-w-4xl mx-auto mb-4"),
		Div(Class("flex flex-col"),
			H2(
				Class("mb-2"),
				g.Text("Oliver Butler 🏔️"),
			),
			Nav(Class("max-w-4xl flex items-center gap-2"),
				g.Group(navItemNodes),
			),
		),
		Div(
			Class("mt-8 flex flex-col gap-2"),
			g.If(user.IsLoggedIn,
				A(Href("/logout"), Class(ButtonStyle), g.Text("Logout"))),
			g.If(user.IsLoggedIn,
				A(Href("/photos/manage"), Class(ButtonStyle), g.Text("Photos Manage"))),
			g.If(!user.IsLoggedIn,
				A(Href(app.Users.GetOAuthAuthorizationUrl()), Class(ButtonStyle), g.Text("Login")),
			),
		),
	)
}

func Page(body g.Node, extraHead ...g.Node) g.Node {
	headContent := []g.Node{
		TitleEl(g.Text("Oliver Butler")),
		Meta(Name("viewport"), Content("width=device-width, user-scalable=no, initial-scale=1.0, maximum-scale=1.0, minimum-scale=1.0")),
		Link(Rel("stylesheet"), Href("/static/output.css")),
		Script(Src("https://unpkg.com/htmx.org@1.9.5/dist/htmx.min.js")),
		Script(Type("module"), Src("/static/olly.js")),
	}

	if os.Getenv("ENV") != "production" {
		headContent = append(headContent, Script(Src("/static/dev-reload.js")))
	}

	headContent = append(headContent, extraHead...)

	return HTML(
		Class("px-4 prose prose-invert max-w-none"),
		Lang("en"),
		Head(g.Group(headContent)),
		Body(
			hx.Boost("swap"),
			body,
		),
	)
}

func SucceessBanner(message string) g.Node {
	return Div(Class("bg-green-100 border-l-4 border-green-500 text-green-700 p-4 mt-4"), g.Text(message))
}
