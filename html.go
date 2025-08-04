package main

import (
	"fmt"
	"time"

	. "github.com/chasefleming/elem-go" //nolint
	a "github.com/chasefleming/elem-go/attrs"
	x "github.com/chasefleming/elem-go/htmx"
)

var (
	dateFormat     string = "Monday 02. January 2006"
	dateTimeFormat string = "Monday 02. January 2006 15:04"
)

func BasePage(props a.Props, children ...Node) *Element {
	content := Html(a.Props{
		a.Lang: "en",
	},
		Head(nil,
			Meta(a.Props{
				a.Charset: "utf-8",
			}),
			Meta(a.Props{
				a.Name:    "viewport",
				a.Content: "initial-scale=1,maximum-scale=1,user-scalable=no",
			}),
			Title(nil, Text("hvor")),
			Link(a.Props{
				a.Rel:  "stylesheet",
				a.Href: "static/tailwind.css",
			}),
			Link(a.Props{
				a.Rel:  "stylesheet",
				a.Href: "https://api.mapbox.com/mapbox-gl-js/v3.0.0-beta.1/mapbox-gl.css",
			}),
			Script(a.Props{
				a.Src: "https://api.mapbox.com/mapbox-gl-js/v3.0.0-beta.1/mapbox-gl.js",
			}),
			Script(a.Props{
				a.Src: "https://unpkg.com/@turf/turf@6/turf.min.js",
			}),
			Script(a.Props{
				a.Src:             "https://umami.kradalby.no/script.js",
				a.Async:           "true",
				"data-website-id": "0de65a1e-5275-4e39-a78e-364e704c0867",
			}),
			Script(a.Props{a.Src: "https://unpkg.com/htmx.org@1.9.10"}),
		),
		Body(props,
			children...,
		),
	)

	return content
}

func hvorPage(p *page, mapboxToken string, lastFetch time.Time) *Element {
	var mapElement, mapScript *Element
	
	if p.Current.Location != nil {
		mapElement = Div(a.Props{
			a.ID:    "map",
			a.Class: "mt-4 h-72",
		})
		
		mapScript = Script(nil, Text(fmt.Sprintf(`
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
		)))
	} else {
		mapElement = Div(
			a.Props{
				a.Class: "mt-4 h-72 bg-gray-100 rounded-lg flex items-center justify-center",
			},
			P(
				a.Props{
					a.Class: "text-gray-500 text-lg",
				},
				Text("Unknown whereabouts"),
			),
		)
		mapScript = nil
	}
	
	return BasePage(nil,
		Div(
			a.Props{
				a.Class: "w-full md:w-2/3 lg:w-1/2 mx-auto",
			},
			Nav(
				a.Props{},
				A(
					a.Props{
						a.Href: "/",
					},
					Span(
						a.Props{
							a.Class: "p-4 flex items-center",
						},
						Img(
							a.Props{
								a.Class: "h-12 md:h-16 mr-4",
								a.Src:   "./static/location.svg",
							}),
						H1(
							a.Props{
								a.Class: "text-3xl md:text-4xl text-gray-700 uppercase",
							}, Text("Hvor")),
					),
				)),
			Main(
				a.Props{
					a.Class: "px-4 py-6",
				},
				mapElement,
				event(p.Current),
				Div(nil,
					H2(
						a.Props{
							a.Class: "text-2xl md:text-3xl text-gray-600 mt-12",
						}, Text("Next")),
					Div(nil,
						events(p.Future, "future", 0, 5)...),
				),
				Div(nil,
					H2(
						a.Props{
							a.Class: "text-2xl md:text-3xl text-gray-600 mt-12",
						}, Text("Past")),
					Div(nil, events(p.Past, "past", 0, 5)...),
				),
			),
			Footer(
				a.Props{
					a.Class: "px-4 py-6 text-sm text-gray-400",
				},
				Text(fmt.Sprintf("Last updated: %s", lastFetch.Format(dateTimeFormat)))),
			mapScript,
		),
	)
}

func event(pe pageEvent) *Element {
	return Div(
		a.Props{
			a.Class: "mt-5",
		},
		P(
			a.Props{
				a.Class: "font-bold text-xl",
			},
			Text(pe.Summary)),
		Div(
			a.Props{a.Class: "text-gray-700 mt-1"},
			TransformEach(pe.Description, func(s string) Node {
				return P(nil, Text(s))
			})...,
		),
		Div(
			a.Props{
				a.Class: "flex justify-end flex-col md:flex-row mt-4 text-gray-600 text-right",
			},
			P(a.Props{a.Class: "text-sm"}, Text(pe.From.Format(dateFormat))),
			P(a.Props{a.Class: "text-sm md:mx-1"}, Text("to")),
			P(a.Props{a.Class: "text-sm"}, Text(pe.To.Format(dateFormat))),
		),
	)
}

func events(es pageEvents, typ string, from, to int) []Node {
	if to > len(es) {
		to = len(es)
	}

	events := TransformEach(es, func(pe pageEvent) Node {
		return event(pe)
	})

	more := If[Node](to != len(es), Div(a.Props{
		a.ID:       fmt.Sprintf("replaceMe%s", typ),
		x.HXGet:    fmt.Sprintf("/%s?from=%d&to=%d", typ, to, to+5),
		x.HXTarget: fmt.Sprintf("#replaceMe%s", typ),
		x.HXSwap:   "outerHTML",
		a.Class:    "italic text-blue-400 underline",
	}, Text("load more...")), None())

	return append(events[from:to], more)
}
