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
			Div(Class("p-4 text-center text-red-500"), g.Text("Error loading photos")),
		))
	}

	photoTiles := []g.Node{}

	for _, photo := range photos {
		aspectRatio := float64(photo.Width) / float64(photo.Height)

		photoTiles = append(photoTiles, Div(
			Class("photo-item"),
			A(Href("/photos/"+photo.ID),
				Img(Src("/api/photos/"+photo.ID+"?quality=medium"),
					g.Attr("blur-hash", photo.BlurHash),
					g.Attr("data-width", fmt.Sprint(photo.Width)),
					g.Attr("data-height", fmt.Sprint(photo.Height)),
					Class("w-full rounded-md"),
					Style(fmt.Sprintf("aspect-ratio: %.2f;", aspectRatio)),
					Alt(photo.Name),
					Loading("lazy"),
				),
			),
		))
	}

	return components.Page(Div(
		components.NavBar("/photos", app, user),
		Div(Class("max-w-4xl mx-auto p-4"),
			Div(ID("masonry-grid"), Class("relative"),
				g.Group(photoTiles),
			),
		),
	))
}
