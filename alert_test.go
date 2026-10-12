package csw

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func wantInvalid(t *testing.T, err error, part string) {
	t.Helper()
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), part) {
		t.Errorf("error = %v, want invalid containing %q", err, part)
	}
}

func TestWidgetAlertValidation(t *testing.T) {
	t.Parallel()
	fields := []Prop{
		{Key: "snd", Label: "Sound", Kind: PropMedia, Accept: []string{MediaAudio}},
		{Key: "pic", Label: "Pic", Kind: PropMedia, Accept: []string{MediaImage}},
		{Key: "txt", Label: "Txt", Kind: PropText},
	}
	long := strings.Repeat("x", maxAlertVarValue+1)
	tests := []struct {
		name string
		a    *WidgetAlert
		want string
	}{
		{"nil", nil, ""},
		{"ok", &WidgetAlert{Events: []string{"twitch.follow", "twitch.cheer"}, Hold: "7s", Sound: "snd",
			Samples: map[string]map[string]string{"twitch.cheer": {"user": "A"}}}, ""},
		{"star", &WidgetAlert{Events: []string{"*"}, Samples: map[string]map[string]string{"twitch.raid": {"n": "1"}}}, ""},
		{"no events", &WidgetAlert{}, "events"},
		{"too many", &WidgetAlert{Events: slices.Repeat([]string{"a"}, 40)}, "events"},
		{"bad event", &WidgetAlert{Events: []string{"Twitch Follow"}}, "lowercase"},
		{"dup event", &WidgetAlert{Events: []string{"a.b", "a.b"}}, "twice"},
		{"star plus", &WidgetAlert{Events: []string{"*", "a.b"}}, "nothing else"},
		{"stray sample", &WidgetAlert{Events: []string{"a.b"}, Samples: map[string]map[string]string{"c.d": {}}}, "not one of"},
		{"bad sample event", &WidgetAlert{Events: []string{"*"}, Samples: map[string]map[string]string{"C D": {}}}, "malformed"},
		{"bad var", &WidgetAlert{Events: []string{"a.b"}, Samples: map[string]map[string]string{"a.b": {"Bad Key": "x"}}}, "sample variable"},
		{"long var", &WidgetAlert{Events: []string{"a.b"}, Samples: map[string]map[string]string{"a.b": {"user": long}}}, "sample variable"},
		{"many vars", &WidgetAlert{Events: []string{"a.b"}, Samples: map[string]map[string]string{"a.b": manyVars()}}, "at most"},
		{"many samples", &WidgetAlert{Events: []string{"*"}, Samples: manySamples()}, "samples for at most"},
		{"hold text", &WidgetAlert{Events: []string{"a.b"}, Hold: "soon"}, "hold"},
		{"hold zero", &WidgetAlert{Events: []string{"a.b"}, Hold: "0s"}, "hold"},
		{"hold long", &WidgetAlert{Events: []string{"a.b"}, Hold: "11m"}, "hold"},
		{"hold max", &WidgetAlert{Events: []string{"a.b"}, Hold: "10m"}, ""},
		{"sound missing", &WidgetAlert{Events: []string{"a.b"}, Sound: "nope"}, "sound"},
		{"sound image", &WidgetAlert{Events: []string{"a.b"}, Sound: "pic"}, "sound"},
		{"sound text", &WidgetAlert{Events: []string{"a.b"}, Sound: "txt"}, "sound"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkWidgetAlert(tt.a, fields)
			if tt.want == "" {
				if err != nil {
					t.Errorf("error = %v", err)
				}
				return
			}
			wantInvalid(t, err, tt.want)
		})
	}
}

func manyVars() map[string]string {
	m := map[string]string{}
	for i := range maxAlertVars + 1 {
		m["v"+strings.Repeat("a", i)] = "x"
	}
	return m
}

func manySamples() map[string]map[string]string {
	m := map[string]map[string]string{}
	for i := range maxAlertEvents + 1 {
		m["e"+strings.Repeat("a", i)] = nil
	}
	return m
}
