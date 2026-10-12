package csw

import (
	"regexp"
	"slices"
	"time"
)

// WidgetAlert describes how a widget presents alerts. Limits: see
// checkWidgetAlert.
type WidgetAlert struct {
	// Events are the bus event types it styles ("twitch.follow", …), or
	// ["*"] for any.
	Events []string `json:"events"`
	// Samples are sample variables per event type, for previews.
	Samples map[string]map[string]string `json:"samples,omitempty"`
	// Hold is how long the alert stays, as a Go duration of at most 10m;
	// the alert wizard's default.
	Hold string `json:"hold,omitempty"`
	// Sound is the key of a media field (accepting audio) holding the
	// alert's sound.
	Sound string `json:"sound,omitempty"`
}

// checkWidgetAlert validates a widget's alert block against its fields.
func checkWidgetAlert(a *WidgetAlert, fields []Prop) error {
	if a == nil {
		return nil
	}
	star, err := checkAlertEvents(a.Events)
	if err != nil {
		return err
	}
	if err := checkAlertSamples(a.Samples, a.Events, star); err != nil {
		return err
	}
	if a.Hold != "" {
		d, err := time.ParseDuration(a.Hold)
		if err != nil || d <= 0 || d > maxAlertHold {
			return invalidf("alert: hold %q must be a duration up to %s", a.Hold, maxAlertHold)
		}
	}
	if a.Sound != "" {
		i := slices.IndexFunc(fields, func(p Prop) bool { return p.Key == a.Sound })
		if i < 0 || fields[i].Kind != PropMedia || !slices.Contains(fields[i].Accept, MediaAudio) {
			return invalidf("alert: sound %q must name a media field that accepts audio", a.Sound)
		}
	}
	return nil
}

// checkAlertEvents validates the event list and reports whether it is ["*"].
func checkAlertEvents(events []string) (star bool, err error) {
	if len(events) < 1 || len(events) > maxAlertEvents {
		return false, invalidf("alert: list 1 to %d events", maxAlertEvents)
	}
	seen := map[string]bool{}
	for _, e := range events {
		if e != "*" && (len(e) > maxWidgetKey || !alertEventRE.MatchString(e)) {
			return false, invalidf("alert: event %q must be lowercase words joined by dots, or *", e)
		}
		if seen[e] {
			return false, invalidf("alert: event %q is listed twice", e)
		}
		seen[e] = true
	}
	if seen["*"] && len(events) > 1 {
		return false, invalidf("alert: * covers every event, so list nothing else")
	}
	return seen["*"], nil
}

// checkAlertSamples validates the sample variables per event type.
func checkAlertSamples(samples map[string]map[string]string, events []string, star bool) error {
	if len(samples) > maxAlertEvents {
		return invalidf("alert: samples for at most %d events", maxAlertEvents)
	}
	for typ, vars := range samples {
		if typ != "*" && !alertEventRE.MatchString(typ) {
			return invalidf("alert: sample event %q is malformed", typ)
		}
		if !star && !slices.Contains(events, typ) {
			return invalidf("alert: samples for %q, which is not one of the events", typ)
		}
		if len(vars) > maxAlertVars {
			return invalidf("alert: at most %d sample variables per event", maxAlertVars)
		}
		for k, v := range vars {
			if !fieldKeyRE.MatchString(k) || len(v) > maxAlertVarValue {
				return invalidf("alert: sample variable %q must be a field-style name with a value under %d bytes", k, maxAlertVarValue)
			}
		}
	}
	return nil
}

// Limits of a widget's alert block.
const (
	maxAlertEvents   = 32
	maxAlertVars     = 32
	maxAlertVarValue = 2048
	maxAlertHold     = 10 * time.Minute
)

var alertEventRE = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)

// cloneAlert deep-copies an alert block.
func cloneAlert(a *WidgetAlert) *WidgetAlert {
	if a == nil {
		return nil
	}
	c := *a
	c.Events = slices.Clone(a.Events)
	if a.Samples != nil {
		c.Samples = make(map[string]map[string]string, len(a.Samples))
		for k, v := range a.Samples {
			m := make(map[string]string, len(v))
			for kk, vv := range v {
				m[kk] = vv
			}
			c.Samples[k] = m
		}
	}
	return &c
}
