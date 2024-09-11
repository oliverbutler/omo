package pages

import (
	"oliverbutler/components"

	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"golang.org/x/net/context"
)

func Error(ctx context.Context, err error) g.Node {
	return components.Page(Div(
		Div(Class("max-w-4xl mx-auto"),
			H1(Class("text-4xl font-bold"), g.Text("Error")),
			P(Class("text-gray-600"), g.Text(err.Error())),
		),
	))
}
