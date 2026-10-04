// Package layout owns cell geometry: rectangles, the hit map that makes a
// terminal mouse-driven, and the box/column maths the shell measures with.
package layout

// Rect is a cell rectangle in terminal coordinates. X and Y are zero-based,
// matching Bubble Tea's mouse events.
type Rect struct{ X, Y, W, H int }

// R builds a Rect.
func R(x, y, w, h int) Rect { return Rect{X: x, Y: y, W: w, H: h} }

// Empty reports a rectangle that can never be hit.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Contains reports whether the cell (x,y) falls inside.
func (r Rect) Contains(x, y int) bool {
	if r.Empty() {
		return false
	}
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Right and Bottom are the exclusive bounds, handy for clipping.
func (r Rect) Right() int  { return r.X + r.W }
func (r Rect) Bottom() int { return r.Y + r.H }

// Rows yields one Rect per line of a block, top to bottom, so a list can
// register a hit target per item without recomputing offsets at the call site.
func (r Rect) Rows(n int) []Rect {
	out := make([]Rect, 0, n)
	for i := 0; i < n && i < r.H; i++ {
		out = append(out, R(r.X, r.Y+i, r.W, 1))
	}
	return out
}

func (r Rect) String() string {
	return itoa(r.X) + "," + itoa(r.Y) + " " + itoa(r.W) + "x" + itoa(r.H)
}

// HitID names an interactive region. It is opaque to the hit map: only the
// component that registered it interprets it.
type HitID string

type hit struct {
	id   HitID
	r    Rect
	data any
}

// HitMap records the geometry of every interactive element while a frame is
// rendered, so a later mouse event can be resolved to a target. Registration
// order is paint order: the last registered hit wins, which is exactly what
// makes a dialog swallow clicks meant for the page underneath it.
//
// The model resets the map at the top of View and fills it as it draws; mouse
// handling then reads the map left by the frame the user is looking at.
type HitMap struct {
	hits []hit
}

// NewHitMap builds an empty map.
func NewHitMap() *HitMap { return &HitMap{} }

// Reset clears the map for a fresh frame without reallocating the slice.
func (h *HitMap) Reset() { h.hits = h.hits[:0] }

// Add registers a clickable region.
func (h *HitMap) Add(id HitID, r Rect, data any) {
	if r.Empty() {
		return
	}
	h.hits = append(h.hits, hit{id: id, r: r, data: data})
}

// Pick resolves a cell to the topmost registered target.
func (h *HitMap) Pick(x, y int) (HitID, any, bool) {
	for i := len(h.hits) - 1; i >= 0; i-- {
		if h.hits[i].r.Contains(x, y) {
			return h.hits[i].id, h.hits[i].data, true
		}
	}
	return "", nil, false
}

// Len and Targets exist so tests can assert that a frame actually registered
// the controls it claims to support — a hit map that stayed empty would
// otherwise look fine on screen and silently kill the mouse.
func (h *HitMap) Len() int { return len(h.hits) }

// Entry is a registered target with its geometry.
type Entry struct {
	ID   HitID
	Rect Rect
	Data any
}

// All lists the targets in paint order, bottom first.
func (h *HitMap) All() []Entry {
	out := make([]Entry, 0, len(h.hits))
	for _, e := range h.hits {
		out = append(out, Entry{ID: e.id, Rect: e.r, Data: e.data})
	}
	return out
}

func (h *HitMap) Targets() []HitID {
	out := make([]HitID, 0, len(h.hits))
	for _, e := range h.hits {
		out = append(out, e.id)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
