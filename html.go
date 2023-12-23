package main

import (
	"fmt"
	"time"

	. "github.com/chasefleming/elem-go" //nolint
	a "github.com/chasefleming/elem-go/attrs"
)

var (
	dateFormat     string = "Monday 02. January 2006"
	dateTimeFormat string = "Monday 02. January 2006 15:04"
)

func Base(props Attrs, children ...Node) *Element {
	content := Html(Attrs{
		a.Lang: "en",
	},
		Head(nil,
			Meta(Attrs{
				a.Charset: "utf-8",
			}),
			Meta(Attrs{
				a.Name:    "viewport",
				a.Content: "initial-scale=1,maximum-scale=1,user-scalable=no",
			}),
			Title(nil, Text("hvor")),
			Link(Attrs{
				a.Rel:  "stylesheet",
				a.Href: "static/tailwind.css",
			}),
			Link(Attrs{
				a.Rel:  "stylesheet",
				a.Href: "https://api.mapbox.com/mapbox-gl-js/v3.0.0-beta.1/mapbox-gl.css",
			}),
			Script(Attrs{
				a.Src: "https://api.mapbox.com/mapbox-gl-js/v3.0.0-beta.1/mapbox-gl.js",
			}),
			Script(Attrs{
				a.Src: "https://unpkg.com/@turf/turf@6/turf.min.js",
			}),
			Script(Attrs{
				a.Src:             "https://umami.kradalby.no/script.js",
				a.Async:           "true",
				"data-website-id": "0de65a1e-5275-4e39-a78e-364e704c0867",
			}),
		),
		Body(props,
			children...,
		),
	)

	return content
}

func hvorPage(p *page, mapboxToken string, lastFetch time.Time) *Element {
	return Base(nil,
		Div(
			Attrs{
				a.Class: "w-full md:w-2/3 lg:w-1/2 mx-auto",
			},
			Nav(
				Attrs{},
				A(
					Attrs{
						a.Href: "/",
					},
					Span(
						Attrs{
							a.Class: "p-4 flex items-center",
						},
						Img(
							Attrs{
								a.Class: "h-12 md:h-16 mr-4",
								a.Src:   "./static/location.svg",
							}),
						H1(
							Attrs{
								a.Class: "text-3xl md:text-4xl text-gray-700 uppercase",
							}, Text("Hvor")),
					),
				)),
			Main(
				Attrs{
					a.Class: "px-4 py-6",
				},
				Div(Attrs{
					a.ID:    "map",
					a.Class: "mt-4 h-72",
				}),
				event(p.Current),
				Div(nil,
					H2(
						Attrs{
							a.Class: "text-2xl md:text-3xl text-gray-600 mt-12",
						}, Text("Next")),
					Div(nil,
						events(p.Future)...),
				),
				Div(nil,
					H2(
						Attrs{
							a.Class: "text-2xl md:text-3xl text-gray-600 mt-12",
						}, Text("Past")),
					Div(nil, events(p.Past)...),
				),
			),
			Footer(
				Attrs{
					a.Class: "px-4 py-6 text-sm text-gray-400",
				},
				Text(fmt.Sprintf("Last updated: %s", lastFetch.Format(dateTimeFormat)))),
			Script(nil, Text(fmt.Sprintf(`
mapboxgl.accessToken = '%s';

let center = [%s, %s]
const map = new mapboxgl.Map({
  container: 'map',
  style: 'mapbox://styles/mapbox/streets-v12',
  center: center,
  scrollZoom: false,
  zoom: 9,
  minZoom: 9,
});

map.on('load', function() {
  let radius = %f;
  let options = {steps: 4, units: 'kilometers', properties: {}};
  let circle = turf.circle(center, radius, options);
  console.log(circle.geometry.coordinates);

  map.fitBounds(new mapboxgl.LngLatBounds(circle.geometry.coordinates[0][0], circle.geometry.coordinates[0][2]), {padding: 50});
})
`,
				mapboxToken,
				p.Current.Location.Longitude,
				p.Current.Location.Latitude,
				p.Current.Location.Radius/1000,
			))),
		),
	)
}

func event(pe pageEvent) *Element {
	return Div(
		Attrs{
			a.Class: "mt-5",
		},
		P(
			Attrs{
				a.Class: "font-bold text-xl",
			},
			Text(pe.Summary)),
		Div(
			Attrs{a.Class: "text-gray-700 mt-1"},
			TransformEach(pe.Description, func(s string) Node {
				return P(nil, Text(s))
			})...,
		),
		Div(
			Attrs{
				a.Class: "flex justify-end flex-col md:flex-row mt-4 text-gray-600 text-right",
			},
			P(Attrs{a.Class: "text-sm"}, Text(pe.From.Format(dateFormat))),
			P(Attrs{a.Class: "text-sm md:mx-1"}, Text("to")),
			P(Attrs{a.Class: "text-sm"}, Text(pe.To.Format(dateFormat))),
		),
	)
}

func events(es pageEvents) []Node {
	return TransformEach(es, func(pe pageEvent) Node {
		return event(pe)
	})
}
