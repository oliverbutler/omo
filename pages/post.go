package pages

import (
	"fmt"
	"oliverbutler/components"
	"oliverbutler/lib"
	"oliverbutler/lib/users"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"golang.org/x/net/context"
)

func Post(ctx context.Context, app *lib.App, user *users.UserContext, slug string) g.Node {
	post, err := app.Blog.GetPost(ctx, slug)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/post/"+slug, app, user),
			Div(Class("max-w-4xl mx-auto"),
				P(g.Text("Error loading post"))),
		))
	}

	return components.Page(Div(
		components.NavBar("/post/"+slug, app, user),
		Div(Class("max-w-4xl mx-auto"),
			H1(Class("text-4xl font-bold"), g.Text(post.Title)),
			P(Class("text-gray-600"), g.Text(post.PubDate.FormattedString())),
			Img(Src(post.HeroImage), Class("w-full rounded-md"), Style("view-transition-name: hero-image-"+post.Slug)),
			Div(Class("mt-4"),
				g.Raw(string(post.Content)),
			),
		),
	), g.Raw(fmt.Sprintf("<style>%s</style>", app.Blog.GetChromaCSS())),
	)
}
