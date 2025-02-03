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

func PhotoPage(ctx context.Context, app *lib.App, user *users.UserContext, id string) g.Node {
	photo, err := app.Photos.GetPhoto(ctx, id)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/photos/"+photo.ID, app, user),
		))
	}

	return components.Page(Div(
		Div(Class("max-w-6xl mx-auto px-4 py-4"),
			Div(Class("flex flex-row justify-between items-center mb-4"),
				H3(Class("text-lg"), g.Text(photo.Name)),
				A(Href("/photos"),
					Class("hover:text-gray-900 transition-colors"),
					g.Text("← Back to photos"),
				),
			),
			A(
				Href("/api/photos/"+photo.ID+"?quality=original"),
				Class("block rounded-lg cursor-zoom-in"),
				Img(
					Src("/api/photos/"+photo.ID+"?quality=large"),
					g.Attr("blur-hash", photo.BlurHash),
					g.Attr("data-width", fmt.Sprint(photo.Width)),
					g.Attr("data-height", fmt.Sprint(photo.Height)),
					Class("max-h-[85vh] w-auto mx-auto rounded-lg hover:opacity-95 transition-opacity"),
					Alt(photo.Name),
				),
			),
		),
	))
}
