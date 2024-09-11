package pages

import (
	"oliverbutler/components"
	"oliverbutler/lib"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"golang.org/x/net/context"
)

func Index(ctx context.Context, app *lib.App) g.Node {
	posts, err := app.Blog.GetAllPosts(ctx)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/"),
			Div(Class("max-w-4xl mx-auto"),
				P(g.Text("Error loading posts"))),
		))
	}

	blogTiles := []g.Node{}

	for _, post := range posts {
		blogTiles = append(blogTiles, Div(Class("bg-neutral-950 p-4 rounded-md"),
			Img(Src(post.HeroImage), Class("w-full rounded-md"), Style("view-transition-name: hero-image-"+post.Slug)),
			H3(Class("text-2xl font-bold"), A(Href("/post/"+post.Slug), g.Text(post.Title))),
			Sub(Class("text-gray-200"), g.Text(post.PubDate.FormattedString())),
			P(Class("text-gray-100"), g.Text(post.Description)),
		))
	}

	return components.Page(Div(
		components.NavBar("/"),
		Div(Class("max-w-4xl mx-auto grid grid-cols-4 gap-4"),
			g.Group(blogTiles),
		),
	))
}
