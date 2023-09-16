package main

import (
	"flag"
	"log"
	"os"

	"github.com/kradalby/kraweb"
)

const defaultHostname = "hvor"

var (
	calendarURL = flag.String(
		"calendar-url",
		getEnv("HVOR_CALENDAR_URL", ""),
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
		"form-tokens",
		getEnv("HVOR_FROM_TOKENS", ""),
		"Comma separated list for access and tracking",
	)

	mapboxToken = flag.String(
		"mapbox-token",
		getEnv("HVOR_MAPBOX_TOKEN", ""),
		"Token for Mapbox API access",
	)
)

func main() {
	flag.Parse()

	// fromTokens := strings.Split(*fromTokensStr, ",")

	logger := log.New(os.Stdout, "hvor: ", log.LstdFlags)

	k := kraweb.NewKraWeb(
		*hostname,
		*tailscaleKeyPath,
		*controlURL,
		*verbose,
		*localAddr,
		logger,
	)

	k.ListenAndServe()
}
