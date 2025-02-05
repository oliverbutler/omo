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
			Div(Class("flex items-center justify-center gap-8 mt-4 text-gray-600 text-sm"),
				// Aperture
				g.If(photo.Aperature != "",
					Div(Class("flex items-center gap-2"),
						g.Raw(`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="12" cy="12" r="10"/>
							<circle cx="12" cy="12" r="4"/>
						</svg>`),
						g.Text(photo.Aperature),
					),
				),
				// Shutter Speed
				g.If(photo.ShutterSpeed != "",
					Div(Class("flex items-center gap-2"),
						g.Raw(`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M12 2v2m0 16v2M4 12H2m20 0h-2m-2.05-5.95l-1.414 1.414M5.464 5.464L4.05 4.05m14.486 14.486l1.414 1.414M5.464 18.536l-1.414 1.414"/>
						</svg>`),
						g.Text(photo.ShutterSpeed),
					),
				),
				// ISO
				g.If(photo.ISO != "",
					Div(Class("flex items-center gap-2"),
						g.Raw(`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M7 7h10v10H7z"/>
							<path d="M12 4v3m0 10v3m-8-8h3m10 0h3"/>
						</svg>`),
						g.Text(photo.ISO),
					),
				),
				// Focal Length
				g.If(photo.FocalLength != "",
					Div(Class("flex items-center gap-2"),
						g.Raw(`<svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M4 4h16v6H4zm4 6v10m8-10v10"/>
						</svg>`),
						g.Text(photo.FocalLength),
					),
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
