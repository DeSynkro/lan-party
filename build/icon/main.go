// Command icon draws the app icon so it can be regenerated rather than being a
// mystery PNG in the repo.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 512

func main() {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	background := color.NRGBA{R: 0x14, G: 0x15, B: 0x1a, A: 0xff}
	accent := color.NRGBA{R: 0x22, G: 0xd3, B: 0xee, A: 0xff}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, background)
		}
	}

	// Rounded border, one pixel of soft edge so it does not look torn.
	drawRoundedBorder(img, 32, 32, size-32, size-32, 96, accent)

	// A keyhole: the thing you look through to see who is at the door.
	cx, cy := float64(size)/2, 210.0
	drawCircle(img, cx, cy, 62, accent)
	drawRect(img, cx-22, cy+30, cx+22, 300, accent)

	f, err := os.Create("build/icon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func drawCircle(img *image.RGBA, cx, cy, r float64, c color.NRGBA) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			if d <= r {
				img.Set(x, y, c)
			}
		}
	}
}

func drawRect(img *image.RGBA, x0, y0, x1, y1 float64, c color.NRGBA) {
	for y := int(y0); y <= int(y1); y++ {
		for x := int(x0); x <= int(x1); x++ {
			if x >= 0 && y >= 0 && x < size && y < size {
				img.Set(x, y, c)
			}
		}
	}
}

func drawRoundedBorder(img *image.RGBA, x0, y0, x1, y1, r float64, c color.NRGBA) {
	for y := int(y0); y <= int(y1); y++ {
		for x := int(x0); x <= int(x1); x++ {
			px, py := float64(x), float64(y)
			if !roundedContains(px, py, x0, y0, x1, y1, r) {
				continue
			}
			if roundedContains(px, py, x0+14, y0+14, x1-14, y1-14, r-14) {
				continue
			}
			img.Set(x, y, c)
		}
	}
}

func roundedContains(px, py, x0, y0, x1, y1, r float64) bool {
	if px < x0 || px > x1 || py < y0 || py > y1 {
		return false
	}
	if px < x0+r && py < y0+r {
		return math.Hypot(px-(x0+r), py-(y0+r)) <= r
	}
	if px > x1-r && py < y0+r {
		return math.Hypot(px-(x1-r), py-(y0+r)) <= r
	}
	if px < x0+r && py > y1-r {
		return math.Hypot(px-(x0+r), py-(y1-r)) <= r
	}
	if px > x1-r && py > y1-r {
		return math.Hypot(px-(x1-r), py-(y1-r)) <= r
	}
	return true
}
