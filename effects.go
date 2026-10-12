package csw

// Text effects: one Effects value styles a piece of text beyond its font,
// size and color. Every text-bearing widget has a PropEffects prop per
// text part (Text: "effects"; Alert: "headlineEffects" and
// "messageEffects"; Latest, Counter, Goal, Event list, Chat: "effects").
// The page draws them all with one component, so an effect added here
// works everywhere. The zero value is plain text.

// Fill kinds.
const (
	FillSolid  = "solid"  // the widget's color
	FillLinear = "linear" // a linear gradient (Angle, Stops)
	FillRadial = "radial" // a radial gradient (Stops)
	FillMedia  = "media"  // a library image or GIF seen through the letters
)

// Decoration lines and styles.
const (
	LineNone      = "none"
	LineUnderline = "underline"
	LineThrough   = "line-through"
	LineOverline  = "overline"
	LineSolid     = "solid"
	LineWavy      = "wavy"
	LineDotted    = "dotted"
	LineDashed    = "dashed"
	LineDouble    = "double"
)

// Layout fits.
const (
	FitNone   = "none"   // text keeps its size
	FitShrink = "shrink" // the size shrinks (down to MinSize) until the text fits the box
)

// Animation kinds.
const (
	AnimNone       = "none"
	AnimPulse      = "pulse"      // whole text grows and shrinks
	AnimFloat      = "float"      // whole text bobs up and down
	AnimShake      = "shake"      // whole text shakes
	AnimShine      = "shine"      // a light sweeps across
	AnimRainbow    = "rainbow"    // the gradient (or hue) cycles
	AnimWave       = "wave"       // letters rise and fall in turn
	AnimBounce     = "bounce"     // letters hop in turn
	AnimJitter     = "jitter"     // letters twitch
	AnimTypewriter = "typewriter" // letters appear one by one, then hold
)

// Effects limits.
const (
	MaxStops        = 5
	MaxOutlines     = 2
	MaxShadows      = 6
	MaxEffectsBytes = 8 << 10
)

// Blend modes are CSS mix-blend-mode values: normal, multiply, screen,
// overlay, darken, lighten, color-dodge, color-burn, hard-light,
// soft-light, difference, exclusion, hue, saturation, color, luminosity.

// Effects are a text part's effects. Colors are "#rrggbb" or "#rrggbbaa";
// lengths are canvas pixels.
type Effects struct {
	Fill       Fill       `json:"fill"`
	Outlines   []Outline  `json:"outlines"` // drawn around the letters, the first innermost
	Shadows    []Shadow   `json:"shadows"`  // drawn in order, the first on top
	Glow       Glow       `json:"glow"`
	Extrude    Extrude    `json:"extrude"`
	Highlight  Highlight  `json:"highlight"`
	Decoration Decoration `json:"decoration"`
	Blend      string     `json:"blend"` // "normal" when unset
	Layout     TextLayout `json:"layout"`
	Animation  Animation  `json:"animation"`
}

// Fill paints the letters.
type Fill struct {
	Kind  string `json:"kind"`  // a Fill* kind; "" or FillSolid uses the widget's color
	Angle int    `json:"angle"` // FillLinear: degrees, 0 = bottom to top, 90 = left to right
	Stops []Stop `json:"stops"` // gradients: 2 to MaxStops
	Media string `json:"media"` // FillMedia: an image "media-…"
}

// Stop is one gradient color at a position from 0 to 100 percent.
type Stop struct {
	Color string `json:"color"`
	At    int    `json:"at"`
}

// Outline is a stroke around the letters (0 < Width ≤ 50).
type Outline struct {
	Width int    `json:"width"`
	Color string `json:"color"`
}

// Shadow is a drop shadow (|X|,|Y| ≤ 200, 0 ≤ Blur ≤ 200).
type Shadow struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Blur  int    `json:"blur"`
	Color string `json:"color"`
}

// Glow is a soft light around the letters; Size 0 is off (≤ 200),
// Strength 1 to 5 layers.
type Glow struct {
	Size     int    `json:"size"`
	Strength int    `json:"strength"`
	Color    string `json:"color"`
}

// Extrude is a 3D block below the letters: Depth steps of 1 px (0 is off,
// ≤ 60) toward Angle degrees (0 = down, 90 = right… like shadows).
type Extrude struct {
	Depth int    `json:"depth"`
	Angle int    `json:"angle"`
	Color string `json:"color"`
}

// Highlight is a band behind each line of text, like a marker; Color "" or
// fully transparent is off.
type Highlight struct {
	Color  string `json:"color"`
	PadX   int    `json:"padX"`   // 0 to 200
	PadY   int    `json:"padY"`   // 0 to 200
	Radius int    `json:"radius"` // 0 to 200
}

// Decoration is a line under, through or over the text.
type Decoration struct {
	Line      string `json:"line"`      // a Line* line; "" = none
	Style     string `json:"style"`     // a Line* style; "" = solid
	Color     string `json:"color"`     // "" = the text color
	Thickness int    `json:"thickness"` // 0 = automatic, ≤ 50
	Offset    int    `json:"offset"`    // underline offset, -50 to 50
}

// TextLayout shapes the text in its box.
type TextLayout struct {
	Fit      string `json:"fit"`      // FitNone or FitShrink
	MinSize  int    `json:"minSize"`  // FitShrink's smallest size, 4 to 1000 (0 = 12)
	Ellipsis bool   `json:"ellipsis"` // one line, cut with "…"
	Balance  bool   `json:"balance"`  // even line lengths
	Skew     int    `json:"skew"`     // degrees, -45 to 45
	Stretch  int    `json:"stretch"`  // horizontal scale in percent, 50 to 200 (0 = 100)
	Vertical bool   `json:"vertical"` // top to bottom
}

// Animation loops while the text is shown.
type Animation struct {
	Kind      string  `json:"kind"`      // an Anim* kind; "" = none
	Speed     float64 `json:"speed"`     // seconds per cycle, 0.2 to 20 (0 = the kind's default)
	Intensity int     `json:"intensity"` // 1 to 10 (0 = 5)
}
