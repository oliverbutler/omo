package pages

import (
	"oliverbutler/components"
	"oliverbutler/lib"
	"oliverbutler/lib/tracing"
	"oliverbutler/lib/users"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"golang.org/x/net/context"
)

func Index(ctx context.Context, app *lib.App, user *users.UserContext) g.Node {
	ctx, span := tracing.OmoTracer.Start(ctx, "Pages.Index")
	defer span.End()

	posts, err := app.Blog.GetAllPosts(ctx)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/", app, user),
			Div(Class("max-w-4xl mx-auto"),
				P(g.Text("Error loading posts"))),
		))
	}

	blogTiles := []g.Node{}

	for _, post := range posts {
		blogTiles = append(blogTiles, Div(Class("bg-gray-50 p-4 rounded-md"),
			Img(Src(post.HeroImage), Class("w-full rounded-md"), Style("view-transition-name: hero-image-"+post.Slug)),
			H3(Class("text-2xl font-bold"), A(Href("/post/"+post.Slug), g.Text(post.Title))),
			Sub(Class("text-gray-900"), g.Text(post.PubDate.FormattedString())),
			P(Class("text-gray-800"), g.Text(post.Description)),
		))
	}

	return components.Page(Div(
		components.NavBar("/", app, user),
		Div(Class("max-w-4xl mx-auto grid sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 grid-cols-1 gap-4"),
			g.Group(blogTiles),
		),
		components.PageFooter(ctx, app),
	))
}
