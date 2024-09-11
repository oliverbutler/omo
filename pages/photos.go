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

func Photos(ctx context.Context, app *lib.App, user *users.UserContext) g.Node {
	photos, err := app.Photos.GetPhotos(ctx)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/photos", app, user),
		))
	}

	photoTiles := []g.Node{}

	for _, photo := range photos {
		photoTiles = append(photoTiles, Div(Class("mb-4 break-inside-avoid"),
			Img(Src(photo.LargePath),
				g.Attr("blur-hash", photo.BlurHash),
				g.Attr("data-width", fmt.Sprint(photo.Width)),
				g.Attr("data-height", fmt.Sprint(photo.Height)),
				Class("w-full rounded-md m-0"),
				Style("aspect-ratio: "+fmt.Sprintf("%f", float64(photo.Width)/float64(photo.Height))),
				Alt(photo.Name),
				Loading("lazy")),
		))
	}

	return components.Page(Div(
		components.NavBar("/photos", app, user),
		Div(Class("max-w-4xl mx-auto columns-1 md:columns-2 xl:columns-3 gap-4"),
			g.Group(photoTiles),
		),
	))
}
