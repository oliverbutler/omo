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
		components.NavBar("/photos", app, user),
		Div(Class("max-w-4xl mx-auto"),
			Div(Class("flex flex-row justify-between items-center gap-4"),
				H3(Class("text-3xl font-bold"), g.Text(photo.Name)),
				Div(Class("flex flex-row gap-2"),
					A(Href("/photos"), Button(Class(components.ButtonStyle), g.Text("Go Back"))),
					A(Href("/api/photos/"+photo.ID+"?quality=original"), Button(Class(components.ButtonStyle), g.Text("Original"))),
				)),
			Img(Src("/api/photos/"+photo.ID+"?quality=large"),
				g.Attr("blur-hash", photo.BlurHash),
				g.Attr("data-width", fmt.Sprint(photo.Width)),
				g.Attr("data-height", fmt.Sprint(photo.Height)),
				Class("w-full rounded-md m-0"),
				Style("aspect-ratio: "+fmt.Sprintf("%f", float64(photo.Width)/float64(photo.Height))),
				Alt(photo.Name),
			),
		),
	))
}
