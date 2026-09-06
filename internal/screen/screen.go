// Package screen detects the ARAM Mayhem augment selection screen by OCR-ing
// the display (macOS, via the ezlol-ocr helper built from tools/ocr).
package screen

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Line is one recognized text line with its normalized position (0..1, top-left origin).
type Line struct {
	Text string  `json:"text"`
	Conf float64 `json:"conf"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
}

// ErrUnavailable means no OCR helper exists on this platform/build.
var ErrUnavailable = errors.New("screen ocr unavailable")

// Reader runs the OCR helper.
type Reader struct {
	path string
	mu   sync.Mutex
	last time.Time
	fail int
}

// New locates the helper: $EZLOL_OCR, then ezlol-ocr next to the executable.
func New() *Reader {
	if runtime.GOOS != "darwin" {
		return &Reader{}
	}
	if p := os.Getenv("EZLOL_OCR"); p != "" {
		return &Reader{path: p}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "ezlol-ocr")
		if _, err := os.Stat(p); err == nil {
			return &Reader{path: p}
		}
	}
	if _, err := os.Stat("bin/ezlol-ocr"); err == nil {
		return &Reader{path: "bin/ezlol-ocr"}
	}
	return &Reader{}
}

// Available reports whether OCR can run.
func (r *Reader) Available() bool { return r.path != "" }

// Read captures the screen and returns recognized lines.
func (r *Reader) Read(ctx context.Context) ([]Line, error) {
	if r.path == "" {
		return nil, ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, r.path).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, errors.New(strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var res struct {
		Lines []Line `json:"lines"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, err
	}
	return res.Lines, nil
}

// Normalize lowercases and strips everything but letters and digits.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hit is a matched candidate with the position of the line it was found on.
type Hit struct {
	Key string
	X   float64
	Y   float64
}

// Match finds which of the candidate names appear in the OCR lines. It returns
// hits in reading order (left to right). A candidate matches when a line equals
// it, contains it, or is within a small edit distance.
func Match(lines []Line, candidates map[string]string) []Hit {
	var hits []Hit
	seen := map[string]bool{}
	for _, l := range lines {
		if l.Conf < 0.3 {
			continue
		}
		t := Normalize(l.Text)
		if len(t) < 4 {
			continue
		}
		for norm, key := range candidates {
			if seen[key] {
				continue
			}
			if t == norm || (len(norm) >= 6 && strings.Contains(t, norm)) || (len(norm) >= 8 && len(t) >= 6 && lev(t, norm) <= 2) {
				seen[key] = true
				hits = append(hits, Hit{key, l.X, l.Y})
			}
		}
	}
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].X < hits[j-1].X; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	return hits
}

// OfferLayout keeps only hits that look like the augment selection cards: in
// the middle band of the screen, on roughly one row, spread horizontally.
// Chat announcements ("X chose Escape Plan") sit bottom-left and stack
// vertically, so they fail the spread/row test.
func OfferLayout(hits []Hit) []Hit {
	var band []Hit
	for _, h := range hits {
		if h.Y >= 0.2 && h.Y <= 0.85 && h.X >= 0.12 && h.X <= 0.88 {
			band = append(band, h)
		}
	}
	if len(band) < 2 {
		return nil
	}
	// Largest group of hits sharing a row (|dy| <= 0.12).
	best := []Hit{}
	for _, a := range band {
		var row []Hit
		for _, b := range band {
			if abs(a.Y-b.Y) <= 0.12 {
				row = append(row, b)
			}
		}
		if len(row) > len(best) {
			best = row
		}
	}
	if len(best) < 2 || best[len(best)-1].X-best[0].X < 0.2 {
		return nil
	}
	return best
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func lev(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
