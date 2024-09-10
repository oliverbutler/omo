package pages

import (
	"fmt"
	"oliverbutler/blog"
	"oliverbutler/components"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"golang.org/x/net/context"
)

func Post(ctx context.Context, slug string) g.Node {
	blogService := blog.NewBlogService()

	posts, err := blogService.GetPost(ctx, slug)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/post/"+slug),
			Div(Class("max-w-4xl mx-auto"),
				P(g.Text("Error loading post"))),
		))
	}

	return components.Page(Div(
		components.NavBar("/post/"+slug),
		Div(Class("max-w-4xl mx-auto"),
			H1(Class("text-4xl font-bold"), g.Text(posts.Title)),
			P(Class("text-gray-600"), g.Text(posts.PubDate.FormattedString())),
			Img(Src(posts.HeroImage), Class("w-full rounded-md")),
			Div(Class("mt-4"),
				g.Raw(string(posts.Content)),
			),
		),
	), g.Raw(fmt.Sprintf("<style>%s</style>", blogService.GetChromaCSS())),
	)
}
