package pages

import (
	"context"
	"encoding/json"
	"fmt"
	"oliverbutler/components"
	"oliverbutler/lib"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
)

func MapsPage(ctx context.Context, app *lib.App) g.Node {
	trips, err := app.Mapping.GetTrips()

	jsonData, err := json.Marshal(trips)
	if err != nil {
		// Handle error (e.g., log it or return an error node)
		return g.Text(fmt.Sprintf("Error marshalling JSON: %v", err))
	}

	return components.Page(
		Div(Class("text-black"), ID("map-container"),
			Div(ID("map")),
			Div(Class("absolute top-0 left-0 p-2 flex flex-row gap-2"),
				A(Href("/"), Class("bg-white p-2 rounded-md text-black"), g.Text("Back Home")),
				Button(ID("map-button"), Class("bg-white p-2 rounded-md"), g.Text("Map")),
				Button(ID("satellite-button"), Class("bg-white p-2 rounded-md"), g.Text("Satellite")),
			),
			Div(ID("trip-selector"),
				Select(ID("trip-select")),
			),
			Div(ID("elevation-graph"),
				Canvas(ID("elevation-chart")),
			),
		),
		Script(g.Attr("type", "application/json"), g.Attr("id", "jsonData"),
			g.Raw(string(jsonData))),
		Script(Src("https://api.mapbox.com/mapbox-gl-js/v3.5.2/mapbox-gl.js")),
		Link(Rel("stylesheet"), Href("https://api.mapbox.com/mapbox-gl-js/v3.5.2/mapbox-gl.css")),
		Script(Src("https://cdn.jsdelivr.net/npm/chart.js")),
		Script(Src("/static/maps.js")),
	)
}
