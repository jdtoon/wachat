package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"rsc.io/qr"
)

// layoutQR renders the QR code encoded from payload at the given size.
// We sample qr.Code.Black(x, y) directly rather than rasterizing to a
// PNG and re-decoding it — keeps memory flat and avoids a per-frame
// PNG decode.
//
// payload is the raw pairing string from whatsmeow's QR channel.
// sizeDp is the target side length; we round to an integer pixel
// module so the QR stays sharp.
func layoutQR(gtx layout.Context, th *Theme, payload string, sizePx int) layout.Dimensions {
	if payload == "" {
		return layout.Dimensions{Size: image.Pt(sizePx, sizePx)}
	}
	code, err := qr.Encode(payload, qr.M)
	if err != nil {
		// Render a placeholder square so the layout doesn't shift; the
		// upstream error path (the QR channel) will surface the failure.
		return blockSquare(gtx, sizePx, color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xFF})
	}
	mod := sizePx / code.Size
	if mod < 1 {
		mod = 1
	}
	side := mod * code.Size

	// Background fill (white-on-canvas square so the QR contrasts even
	// against a dark theme).
	bgRect := image.Rect(0, 0, side, side)
	stkBg := clip.Rect(bgRect).Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stkBg.Pop()

	// Module pass: one clip+fill per black pixel. The number of clips is
	// bounded by code.Size² which is small (≤ ~200²) — well within Gio's
	// per-frame op budget.
	dark := color.NRGBA{R: 0x11, G: 0x14, B: 0x0F, A: 0xFF} // matches Theme.TextPrimary
	_ = th
	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			if !code.Black(x, y) {
				continue
			}
			r := image.Rect(x*mod, y*mod, (x+1)*mod, (y+1)*mod)
			stk := clip.Rect(r).Push(gtx.Ops)
			paint.ColorOp{Color: dark}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			stk.Pop()
		}
	}
	return layout.Dimensions{Size: image.Pt(side, side)}
}

// blockSquare paints a single-color square; used as a placeholder when
// QR encoding fails.
func blockSquare(gtx layout.Context, side int, c color.NRGBA) layout.Dimensions {
	r := image.Rect(0, 0, side, side)
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(side, side)}
}
