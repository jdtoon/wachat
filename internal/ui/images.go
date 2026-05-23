package ui

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/jdtoon/wachat/internal/media"
)

// jpegDecodeFn opens path and decodes it as an image. Used by
// media.Cache. The byte cost passed to the cache is the decoded
// bitmap's footprint (width*height*4) — a coarse estimate but the
// budget is also coarse.
func jpegDecodeFn(path string) (image.Image, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		// image.Decode tries every format registered via init —
		// jpeg / png are blank-imported above. WebP / GIF lands when
		// we add the relevant decoder import.
		if jimg, jerr := jpeg.Decode(f); jerr == nil {
			img = jimg
		} else {
			return nil, 0, fmt.Errorf("decode %q: %w", path, err)
		}
	}
	b := img.Bounds()
	cost := b.Dx() * b.Dy() * 4
	return img, cost, nil
}

// thumbnailBudget is the LRU byte budget for in-RAM decoded
// thumbnails. ~30 MB caps the worst-case memory cost regardless of
// chat length — fits comfortably in the CLAUDE.md §2 "memory
// independent of history size" constraint.
const thumbnailBudget = 30 * 1024 * 1024

// NewThumbnailCache returns a media.Cache wired with the JPEG/PNG
// decoder above and the wachat thumbnail budget. The frame loop
// constructs one of these per session.
func NewThumbnailCache() *media.Cache {
	return media.New(thumbnailBudget, jpegDecodeFn)
}

// layoutThumbnail draws a decoded image into a fixed-size rounded
// rectangle so the bubble's height stays predictable (one of
// docs/design.md §5's hard rules).
//
// Limitation: we paint the image at its native size inside a clip; we
// don't yet scale. WhatsApp's JPEGThumbnail is already small (typically
// ≤ 100px), so the box renders the thumbnail centered with surrounding
// fill. Proper aspect-ratio scaling lands in v0.1.8.x.
func layoutThumbnail(gtx layout.Context, th *Theme, img image.Image, maxDp unit.Dp) layout.Dimensions {
	if img == nil {
		return layout.Dimensions{}
	}
	side := gtx.Dp(maxDp)
	rr := gtx.Dp(th.Radius.Card)
	rect := image.Rect(0, 0, side, side)
	stk := clip.UniformRRect(rect, rr).Push(gtx.Ops)
	// Surface fill behind the image so the rounded corners are flat
	// against the bubble's background.
	paint.ColorOp{Color: th.Palette.Surface}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	paint.NewImageOp(img).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stk.Pop()
	return layout.Dimensions{Size: image.Pt(side, side)}
}
