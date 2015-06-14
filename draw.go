package main

import (
	"image"
	"image/color"
)

type circle struct {
	r int
}

func (c *circle) ColorModel() color.Model {
	return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
	return image.Rect(0, 0, c.r*2, c.r*2)
}

func (c *circle) At(x, y int) color.Color {
	xx, yy, rr := float64(x-c.r), float64(y-c.r), float64(c.r)
	if xx*xx+yy*yy < rr*rr {
		return color.Alpha{255}
	}
	return color.Alpha{0}
}
