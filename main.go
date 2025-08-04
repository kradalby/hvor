package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/kradalby/kraweb"
	"tailscale.com/client/tailscale"
	"tailscale.com/types/logger"
)

const (
	defaultHostname = "hvor"
	refreshPeriod   = 30 * time.Minute
)

var (
	reUnitedStates = regexp.MustCompile(`(\w.+), ([A-Z]{2})n(United States)`)
	reSandefjord   = regexp.MustCompile(`(Sandefjord)n(Sandefjord Municipality), (Norway)`)
)

var (
	calendarURL = flag.String(
		"calendar-url",
		getEnv(
			"HVOR_CALENDAR_URL",
			"",
		),
		"URL to a shared Calendar in iCal format",
	)

	tailscaleKeyPath = flag.String(
		"ts-key-path",
		getEnv("HVOR_TS_KEY_PATH", ""),
		"Path to tailscale auth key",
	)

	hostname = flag.String("ts-hostname", getEnv("HVOR_TS_HOSTNAME", defaultHostname), "")

	controlURL = flag.String(
		"ts-controlurl",
		getEnv("HVOR_TS_CONTROL_SERVER", ""),
		"Tailscale Control server, if empty, upstream",
	)

	verbose = flag.Bool("verbose", getEnvBool("HVOR_VERBOSE", false), "be verbose")

	localAddr = flag.String(
		"listen-addr",
		getEnv("HVOR_LISTEN_ADDR", "localhost:56663"),
		"Local address to listen to",
	)

	monthsFuture = flag.Int(
		"months-future",
		getEnvInt("HVOR_MONTHS_FUTURE", 3),
		"Months to include in future",
	)

	monthsPast = flag.Int(
		"months-past",
		getEnvInt("HVOR_MONTHS_PAST", 6),
		"Months to include from the past",
	)

	fromTokensStr = flag.String(
		"from-tokens",
		getEnv("HVOR_FROM_TOKENS", ""),
		"Comma separated list for access and tracking",
	)

	mapboxToken = flag.String(
		"mapbox-token",
		getEnv("HVOR_MAPBOX_TOKEN", ""),
		"Token for Mapbox API access",
	)

	dev = flag.Bool(
		"dev",
		getEnvBool("HVOR_DEV", false),
		"disable tailscale",
	)
)

func fetchCalendar(url string) (*ics.Calendar, error) {
	var body []byte
	var err error

	if *dev {
		body, err = os.ReadFile("./cal.dump")
		if err != nil {
			return nil, fmt.Errorf("failed to read cal from disk: %w", err)
		}
	} else {
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get calendar: %w", err)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
	}

	cal, err := ics.ParseCalendar(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse calendar: %w", err)
	}

	return cal, nil
}

type pageEvents []pageEvent

func (u pageEvents) Len() int {
	return len(u)
}

func (u pageEvents) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

func (u pageEvents) Less(i, j int) bool {
	return u[i].To.Before(u[j].To)
}

type pageEvent struct {
	From        time.Time
	To          time.Time
	Location    *appleLocation
	Summary     string
	Description []string
}

type page struct {
	Current pageEvent
	Past    pageEvents
	Future  pageEvents
}

type appleLocation struct {
	Title        string
	Radius       float64
	Latitude     string
	Longitude    string
	MapkitHandle string
}

func getAppleLocation(event *ics.VEvent) *appleLocation {
	ret := appleLocation{}

	comp := event.GetProperty(
		ics.ComponentProperty("X-APPLE-STRUCTURED-LOCATION"),
	)
	if comp == nil {
		return nil
	}

	if titles, ok := comp.ICalParameters["X-TITLE"]; ok && len(titles) > 0 {
		ret.Title = sanitiseLocationTitle(titles[0])
	}

	if radiusParams, ok := comp.ICalParameters["X-APPLE-RADIUS"]; ok && len(radiusParams) > 0 {
		if radius, err := strconv.ParseFloat(radiusParams[0], 64); err == nil {
			ret.Radius = radius
		}
	}

	if handles, ok := comp.ICalParameters["X-APPLE-MAPKIT-HANDLE"]; ok && len(handles) > 0 {
		ret.MapkitHandle = handles[0]
	}

	if coordString, found := strings.CutPrefix(comp.Value, "geo:"); found {
		coord := strings.Split(coordString, ",")
		// TODO: Check if this is the right lat long order
		if len(coord) == 2 {
			ret.Latitude = coord[0]
			ret.Longitude = coord[1]
		}
	}

	return &ret
}

func sanitiseLocationTitle(title string) string {
	usMatch := reUnitedStates.FindStringSubmatch(title)

	if len(usMatch) > 2 {
		// Replace state two-letter code with
		// full name.
		if stateName, ok := usc[usMatch[2]]; ok {
			usMatch[2] = stateName
		}

		return strings.Join(usMatch[1:], ", ")
	}

	sfMatch := reSandefjord.FindStringSubmatch(title)

	if len(sfMatch) > 0 {
		return strings.Join(sfMatch[1:], ", ")
	}

	return strings.ReplaceAll(title, "\\n", ", ")
}

func sanatiseCalText(str string) string {
	return strings.ReplaceAll(str, "\\,", ",")
}

func sanitiseDescription(desc string) []string {
	return strings.Split(desc, "\\n")
}

func createPage(cal *ics.Calendar) (*page, error) {
	now := time.Now()

	p := page{
		Past:   make(pageEvents, 0),
		Future: make(pageEvents, 0),
	}

	for _, event := range cal.Events() {
		from, err := event.GetAllDayStartAt()
		if err != nil {
			return nil, err
		}

		to, err := event.GetAllDayEndAt()
		if err != nil {
			return nil, err
		}

		summary := event.GetProperty(ics.ComponentPropertySummary)
		desc := event.GetProperty(ics.ComponentPropertyDescription)

		pe := pageEvent{
			From:        from,
			To:          to,
			Location:    getAppleLocation(event),
			Summary:     sanatiseCalText(summary.Value),
			Description: []string{},
		}

		if desc != nil {
			pe.Description = sanitiseDescription(sanatiseCalText(desc.Value))
		}

		if to.Before(now) {
			p.Past = append(p.Past, pe)

			continue
		}

		if from.After(now) {
			p.Future = append(p.Future, pe)

			continue
		}

		p.Current = pe
	}

	sort.Sort(sort.Reverse(p.Past))
	sort.Sort(p.Future)

	return &p, nil
}

type tokens struct {
	ts []string
}

func parseTokens(str string) tokens {
	list := strings.Split(str, ",")

	return tokens{
		ts: list,
	}
}

func (t *tokens) isValid(token string) bool {
	for _, tok := range t.ts {
		if tok == token {
			return true
		}
	}

	return false
}

type hvor struct {
	url         string
	tokens      tokens
	lastFetch   time.Time
	calPage     *page
	mapboxToken string
	tsLocal     *tailscale.LocalClient
	logf        logger.Logf
}

func (h *hvor) updater() {
	tick := time.Tick(refreshPeriod)

	for {
		<-tick

		err := h.updateCalendar()
		if err != nil {
			h.logf("failed to update calendar data: %s", err)
		}
	}
}

func (h *hvor) updateCalendar() error {
	cal, err := fetchCalendar(h.url)
	if err != nil {
		return err
	}

	p, err := createPage(cal)
	if err != nil {
		return err
	}

	h.calPage = p
	h.lastFetch = time.Now()

	return nil
}

func (h *hvor) isViaTailscale(r *http.Request) bool {
	if h.tsLocal == nil {
		h.logf("no tailscale client is available, connection not coming from tailscale")

		return false
	}

	who, err := h.tsLocal.WhoIs(r.Context(), r.RemoteAddr)
	if err != nil {
		h.logf("failed to find out who connected with tailscale: %s", err)

		return false
	}

	displayName := "Unknown User"
	if who.UserProfile != nil {
		displayName = who.UserProfile.DisplayName
	}
	h.logf("tailscale who: %s", displayName)

	return true
}

func (h *hvor) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from := r.URL.Query().Get("from")
		if !h.isViaTailscale(r) && !h.tokens.isValid(from) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorised, you probably do not have a direct link"))

			return
		}

		// TODO(kradalby): use from for metrics

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(hvorPage(h.calPage, h.mapboxToken, h.lastFetch).Render()))
	})
}

func pager(w http.ResponseWriter, r *http.Request) (int, int, error) {
	fromStr := r.URL.Query().Get("from")

	from, err := strconv.Atoi(fromStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid from"))
	}

	toStr := r.URL.Query().Get("to")
	to, err := strconv.Atoi(toStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid to"))
	}

	return from, to, err
}

func (h *hvor) future() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, to, err := pager(w, r)
		if err != nil {
			return
		}

		evs := events(h.calPage.Future, "future", from, to)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(renderNodeList(evs)))
	})
}

func (h *hvor) past() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, to, err := pager(w, r)
		if err != nil {
			return
		}

		evs := events(h.calPage.Past, "past", from, to)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(renderNodeList(evs)))
	})
}

//go:embed all:static
var staticAssets embed.FS

func main() {
	flag.Parse()

	toks := parseTokens(*fromTokensStr)

	logger := log.New(os.Stdout, "hvor: ", log.LstdFlags)

	k := kraweb.NewKraWeb(
		*hostname,
		*tailscaleKeyPath,
		*controlURL,
		*verbose,
		*localAddr,
		logger,
		!*dev,
	)

	h := hvor{
		url:         *calendarURL,
		tokens:      toks,
		mapboxToken: *mapboxToken,
		logf:        logger.Printf,
	}

	if err := h.updateCalendar(); err != nil {
		log.Fatalf("Failed to get initial calendar: %s", err)
	}

	if localClient := k.TailscaleLocalClient(); localClient != nil {
		h.tsLocal = localClient
	}

	logger.Printf("starting background updater of calendar data, running every %s", refreshPeriod)
	go h.updater()

	staticFS := http.FS(staticAssets)
	fs := http.FileServer(staticFS)
	k.Handle("/static/", fs)

	k.Handle("/", h.handler())
	k.Handle("/future", h.future())
	k.Handle("/past", h.past())

	log.Fatalf("Failed to serve %s", k.ListenAndServe())
}
