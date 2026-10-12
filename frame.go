package csw

// Frames: one FrameStyle value styles a box: its shape, fill, borders,
// brackets, shadows, glow, texture and animation, or an artist's image
// frame (9-slice). The Frame widget is a frame on its own; chat boxes,
// event lists, alerts, goal bars and the latest/counter plates carry frame
// props too (box and row styles), drawn by the same component. The zero
// FrameStyle draws nothing.

// Shape kinds.
const (
	ShapeRect     = "rect"     // Radius per corner
	ShapeCut      = "cut"      // corners cut at 45°, Radius is each cut's size
	ShapeNotch    = "notch"    // a rectangle with a tab (Depth > 0) or notch (< 0) on one edge
	ShapePill     = "pill"     // fully rounded ends
	ShapeSlant    = "slant"    // a parallelogram leaning Slant degrees
	ShapeTicket   = "ticket"   // round bites out of the left and right edges (Radius[0] is the bite)
	ShapeHexagon  = "hexagon"  // pointed left and right ends
	ShapeBubble   = "bubble"   // a rounded speech bubble with a Pointer
	ShapeEllipse  = "ellipse"  // an ellipse, a circle in a square box
	ShapeTriangle = "triangle" // an isosceles triangle pointing up
	ShapeStar     = "star"     // a star with Points tips, Inner percent deep
	ShapePolygon  = "polygon"  // a regular polygon with Points corners
	ShapeHeart    = "heart"    // a heart
	ShapeArrow    = "arrow"    // a block arrow pointing right
)

// Edges.
const (
	EdgeTop    = "top"
	EdgeRight  = "right"
	EdgeBottom = "bottom"
	EdgeLeft   = "left"
)

// FillNone is the fill kind of a transparent frame; the other fill kinds
// (FillSolid, FillLinear, FillRadial and FillMedia) are as for text.
const FillNone = "none"

// Pattern kinds drawn over a frame's fill.
const (
	PatternNone      = "none"
	PatternStripes   = "stripes"
	PatternDots      = "dots"
	PatternGrid      = "grid"
	PatternChecker   = "checker"
	PatternScanlines = "scanlines"
	PatternCarbon    = "carbon"
)

// Border styles.
const (
	BorderSolid  = "solid"
	BorderDashed = "dashed"
	BorderDotted = "dotted"
	BorderDouble = "double"
)

// Texture kinds.
const (
	TextureNone  = "none"
	TextureNoise = "noise"
	TextureGrain = "grain"
)

// 9-slice repeat modes (CSS border-image-repeat).
const (
	RepeatStretch = "stretch"
	RepeatRepeat  = "repeat"
	RepeatRound   = "round"
	RepeatSpace   = "space"
)

// Frame animation kinds.
const (
	FrameAnimNone    = "none"
	FrameAnimRunning = "running" // a light runs around the border
	FrameAnimAnts    = "ants"    // the dashes of a dashed border march
	FrameAnimPulse   = "pulse"   // the glow breathes
	FrameAnimShine   = "shine"   // a light sweeps across the panel
)

// Frame limits.
const (
	MaxRings       = 2
	MaxBoxShadows  = 4
	MaxFrameBytes  = 8 << 10
	MaxFrameLength = 2000 // radius, widths, sizes in canvas pixels
)

// FrameStyle styles a box. Colors are "#rrggbb", "#rrggbbaa" or a theme color
// ("theme:accent"); lengths are canvas pixels.
type FrameStyle struct {
	Shape     Shape          `json:"shape"`
	Fill      FrameFill      `json:"fill"`
	Border    Border         `json:"border"`
	Rings     []Ring         `json:"rings"`    // outside the border, the first nearest
	Brackets  Brackets       `json:"brackets"` // corner brackets
	Shadows   []BoxShadow    `json:"shadows"`
	Glow      Glow           `json:"glow"` // as for text: Size 0 is off
	Bevel     Bevel          `json:"bevel"`
	Texture   Texture        `json:"texture"`
	Image     NineSlice      `json:"image"` // when Image.Media is set the box is drawn from the image instead of Shape, Fill and Border
	Animation FrameAnimation `json:"animation"`
	// Padding keeps content (chat lines, event rows, alert content) this
	// far inside the frame.
	Padding int `json:"padding"`
}

// Shape is the outline of the box. Kind "" means ShapeRect.
type Shape struct {
	Kind    string  `json:"kind"`
	Radius  [4]int  `json:"radius"` // top-left, top-right, bottom-right, bottom-left
	Slant   int     `json:"slant"`  // ShapeSlant: -45 to 45 degrees
	Notch   Notch   `json:"notch"`
	Pointer Pointer `json:"pointer"`
	// Points is the number of tips of ShapeStar or corners of ShapePolygon:
	// 0 (the default, 5 for a star and 6 for a polygon) or 3 to 12.
	Points int `json:"points"`
	// Inner is a ShapeStar's inner radius as a percent of the outer one: 0
	// (the default, 40) or 10 to 90.
	Inner int `json:"inner"`
}

// Notch is ShapeNotch's tab (Depth > 0, sticking out) or notch (Depth < 0,
// cut in) on one edge, centered At percent along it.
type Notch struct {
	Edge  string `json:"edge"`
	At    int    `json:"at"` // 0 to 100
	Width int    `json:"width"`
	Depth int    `json:"depth"` // -MaxFrameLength to MaxFrameLength
}

// Pointer is ShapeBubble's speech pointer on one edge, At percent along it.
type Pointer struct {
	Edge string `json:"edge"`
	At   int    `json:"at"` // 0 to 100
	Size int    `json:"size"`
}

// FrameFill paints the inside of the box. Kind "" or FillNone leaves it
// transparent (a webcam frame); a Pattern can sit on any fill.
type FrameFill struct {
	Kind    string  `json:"kind"`
	Color   string  `json:"color"` // FillSolid
	Angle   int     `json:"angle"` // FillLinear: degrees, 90 = left to right
	Stops   []Stop  `json:"stops"` // gradients: 2 to MaxStops
	Media   string  `json:"media"` // FillMedia: an image "media-…", covering the box
	Pattern Pattern `json:"pattern"`
}

// Pattern is a repeating texture drawn with CSS over the fill.
type Pattern struct {
	Kind  string `json:"kind"`
	Color string `json:"color"`
	Size  int    `json:"size"`  // one repeat, 2 to 400 (0 = 16)
	Angle int    `json:"angle"` // stripes: degrees
}

// Border is the box's edge. Width 0 is no border; with 2 or more Stops it
// is a gradient at Angle. Inset moves it inward (negative: outward).
type Border struct {
	Width int    `json:"width"`
	Style string `json:"style"` // a Border* style; "" = solid
	Color string `json:"color"`
	Angle int    `json:"angle"`
	Stops []Stop `json:"stops"`
	Inset int    `json:"inset"`
}

// Ring is an extra line around the border, Gap pixels outside the one
// before it.
type Ring struct {
	Width int    `json:"width"`
	Gap   int    `json:"gap"`
	Color string `json:"color"`
}

// Brackets draw only the corners of the border, Length pixels along each
// edge (0 is off), Inset pixels inside the box.
type Brackets struct {
	Length int    `json:"length"`
	Width  int    `json:"width"`
	Color  string `json:"color"`
	Inset  int    `json:"inset"`
}

// BoxShadow is a drop shadow, or an inner one when Inset.
type BoxShadow struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Blur   int    `json:"blur"`
	Spread int    `json:"spread"`
	Color  string `json:"color"`
	Inset  bool   `json:"inset"`
}

// Bevel lights the top-left inner edge and darkens the bottom-right one;
// Size 0 is off.
type Bevel struct {
	Size  int    `json:"size"`
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

// Texture is a fine noise or film grain over the box at Opacity percent.
type Texture struct {
	Kind    string `json:"kind"`
	Opacity int    `json:"opacity"` // 0 to 100
}

// NineSlice draws the box from an artist's frame image: Slice cuts the
// image into corners, edges and a center (top, right, bottom, left, in
// image pixels); the corners keep their size and the edges stretch or
// repeat. Width is the drawn border size per side (0 = the slice size);
// Fill also draws the image's center.
type NineSlice struct {
	Media  string `json:"media"`
	Slice  [4]int `json:"slice"`
	Width  [4]int `json:"width"`
	Repeat string `json:"repeat"` // a Repeat* mode; "" = stretch
	Fill   bool   `json:"fill"`
}

// FrameAnimation loops while the frame is shown.
type FrameAnimation struct {
	Kind  string  `json:"kind"`  // a FrameAnim* kind; "" = none
	Speed float64 `json:"speed"` // seconds per cycle, 0.2 to 20 (0 = the kind's default)
	Color string  `json:"color"` // the running light's or shine's color ("" = white)
}
