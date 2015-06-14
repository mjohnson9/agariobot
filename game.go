package main

import (
	"sort"

	"github.com/ajhager/engi"
	"github.com/nightexcessive/agario"
)

type Game struct {
	*engi.Game

	g        *agario.Game
	quitChan chan struct{}

	batch *engi.Batch
	W, H  float32

	cameraX, cameraY float32

	circle engi.Drawable
	font   *engi.Font
}

func (g *Game) Preload() {
	engi.Files.Add("font", "data/font.png")
	g.W, g.H = engi.Width(), engi.Height()
	g.batch = engi.NewBatch(g.W, g.H)
}

const (
	circleSize = 2048
)

func (g *Game) Setup() {
	engi.SetBg(0x2d3739)
	g.font = engi.NewGridFont(engi.Files.Image("font"), 20, 20)
	g.circle = g.getCircleTexture(circleSize / 2)
}

func (g *Game) Render() {
	g.g.Lock()
	defer g.g.Unlock()

	g.batch.Begin()

	g.cameraX, g.cameraY = g.calculateCamera()
	g.renderCells()

	g.batch.End()
}

func (g *Game) Close() {
	close(g.quitChan)
}

func (g *Game) Resize(w, h int) {
	g.W, g.H = float32(w), float32(h)
	g.batch = engi.NewBatch(g.W, g.H)
}

func (g *Game) renderCells() {
	cells := g.getCells()

	for _, c := range cells {
		g.drawCell(c)
	}
}

func (g *Game) drawCell(c *agario.Cell) {
	red, green, blue, alpha := c.Color.RGBA()
	colVal := ((red & 0xFF) << 16) | ((green & 0xFF) << 8) | (blue & 0xFF)
	scale := float32(c.Size) / circleSize
	g.batch.Draw(g.circle, c.Position.X()-g.cameraX, c.Position.Y()-g.cameraY, 0.5, 0.5, scale, scale, 0, colVal, float32(alpha)/255)
}

func (g *Game) getCells() []*agario.Cell {
	cells := make([]*agario.Cell, 0, len(g.g.Cells))

	for _, c := range g.g.Cells {
		cells = append(cells, c)
	}

	cellSlice(cells).Sort()
	return cells
}

func (g *Game) calculateCamera() (float32, float32) {
	var center *agario.Cell
	for id := range g.g.MyIDs {
		cell, ok := g.g.Cells[id]
		if !ok {
			continue
		}

		if center == nil || cell.ID < center.ID {
			center = cell
		}
	}

	if center == nil {
		return 0, 0
	}

	x, y := center.Position.Elem()

	return x - g.W/2, y - g.H/2
}

func (g *Game) getCircleTexture(d int) engi.Drawable {
	img := engi.LoadImage(&circle{d})
	t := engi.NewTexture(img)
	return t
}

// cellSlice attaches the methods of Interface to []int, sorting in increasing order.
type cellSlice []*agario.Cell

func (p cellSlice) Len() int { return len(p) }
func (p cellSlice) Less(i, j int) bool {
	a, b := p[i], p[j]
	if a.Size == b.Size {
		return a.ID < b.ID
	}

	return a.Size < b.Size
}
func (p cellSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p cellSlice) Sort() { sort.Sort(p) }
