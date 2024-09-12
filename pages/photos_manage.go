package pages

import (
	"fmt"
	"oliverbutler/components"
	"oliverbutler/lib"
	"oliverbutler/lib/photos"
	"oliverbutler/lib/users"

	g "github.com/maragudk/gomponents"
	hx "github.com/maragudk/gomponents-htmx"
	. "github.com/maragudk/gomponents/html"
	"golang.org/x/net/context"
)

func PhotoManageTile(photo *photos.Photo) g.Node {
	return Div(ID("image"+photo.ID), Class("bg-gray-900 p-4 flex flex-col  rounded-md"),
		Div(Class("flex flex-row gap-2"),
			Img(Src("/api/photos/"+photo.ID+"?quality=medium"),
				Class("rounded-md m-0 h-52 w-fit"),
				Alt(photo.Name),
				Loading("lazy")),

			Div(
				Class("rounded-md m-0 h-52 w-fit"),
				g.Attr("blur-hash", photo.BlurHash),
				g.Attr("data-width", fmt.Sprint(photo.Width)),
				g.Attr("data-height", fmt.Sprint(photo.Height)),
				Style("aspect-ratio: "+fmt.Sprintf("%f", float64(photo.Width)/float64(photo.Height))),
			),
		),
		Div(Class("flex flex-row gap-2 my-4"),
			Div(Class("flex flex-col gap-2"),
				Span(Class("text-gray-200"), g.Text(photo.Name)),
				Sub(Class("text-gray-200"), g.Text(photo.ID)),
			),
			Div(Class("flex flex-col gap-2"),
				Button(Class("bg-red-800 text-white p-2 rounded-md"), g.Text("Delete"), hx.Delete("/photos/"+photo.ID), hx.Trigger("click"), hx.Confirm("Are you sure you want to delete this photo?"),
					hx.Target("closest #image"+photo.ID), hx.Swap("outerHTML")),
			),
		))
}

func PhotosManage(ctx context.Context, app *lib.App, user *users.UserContext) g.Node {
	photos, err := app.Photos.GetPhotos(ctx)
	if err != nil {
		return components.Page(Div(
			components.NavBar("/photos/manage", app, user),
		))
	}

	photoTiles := []g.Node{}

	for _, photo := range photos {
		photoTiles = append(photoTiles, PhotoManageTile(&photo))
	}

	return components.Page(Div(
		Class("max-w-4xl mx-auto"),
		components.NavBar("/photos", app, user),
		Form(ID("image-upload-form"),
			hx.Post("/photos/upload"),
			hx.Trigger("submit"),
			hx.Encoding("multipart/form-data"),
			hx.Target("#photo-tiles"),
			hx.Swap("afterbegin"),
			hx.Indicator("#progress"),
			Input(Type("file"), Name("photo"), ID("photo-upload"), Accept("image/*"), Multiple(), Class("bg-gray-800 text-white p-2 rounded-md w-full mb-4")),
			Button(Type("submit"), Class("bg-gray-800 text-white p-2 rounded-md w-full"), g.Text("Upload")),
		),
		Div(ID("progress"), Class("htmx-indicator"),
			Div(Class("w-full bg-gray-200 rounded-full h-2.5 dark:bg-gray-700"),
				Div(Class("bg-blue-600 h-2.5 rounded-full"), Style("width: 0%")),
			),
		),
		Div(ID("photo-tiles"), Class("flex flex-col gap-4"),
			g.Group(photoTiles),
		),
	))
}
