package main

import (
	"log"
	"sort"
	"time"

	"github.com/nightexcessive/agario"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	windowWidth  = 1600
	windowHeight = 900
)

func createGame(internalGame *agario.Game) *game {
	g := &game{
		Game:         internalGame,
		nicknameSent: time.Time{},
	}

	g.ai = &AI{
		g: g,
	}

	var err error

	g.window, err = sdl.CreateWindow("agariobot", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, windowWidth, windowHeight, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}

	g.renderer, err = sdl.CreateRenderer(g.window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		panic(err)
	}

	return g
}

type game struct {
	window   *sdl.Window
	renderer *sdl.Renderer

	ai *AI

	nicknameSent      time.Time
	nicknameRetrySent bool

	Game *agario.Game

	//Map *Map
	//NodeList []graph.Node
}

func (g *game) Tick(dt time.Duration) bool {
	if g.Game.Board == nil {
		log.Printf("Waiting for board setup...")
		return true
	} else if g.Game.Board.Top != 0 || g.Game.Board.Left != 0 {
		panic("Board dimensions start somewhere other than (0, 0)")
	}

	/*if g.Map == nil {
		g.Map = createMap(g.Game)
	}*/

	if len(g.Game.MyIDs) == 0 {
		const (
			idResendTime = 1 * time.Second
			idWaitTime   = 3 * time.Second
		)
		if g.nicknameSent.IsZero() {
			log.Printf("Sent my nickname: %s", name)
			g.Game.SendNickname(name)
			g.nicknameSent = time.Now()
			return true
		}
		timeSinceSent := time.Now().Sub(g.nicknameSent)
		if timeSinceSent > idWaitTime {
			log.Printf("Giving up after waiting %s for a new spawn", idWaitTime)
			return false
		} else if timeSinceSent > idResendTime && !g.nicknameRetrySent {
			log.Println("Retrying sending my nickname...")
			g.Game.SendNickname(name)
			g.nicknameRetrySent = true
			return true
		}
		log.Printf("Waiting for my ID...")
		return true
	}

	if !g.nicknameSent.IsZero() {
		g.nicknameSent = time.Time{}
		g.nicknameRetrySent = false
	}

	me := g.getOwnCell()
	if me == nil {
		log.Printf("Can't find own cell")
		return true
	}

	cells := g.getSortedCells()

	g.ai.Update(dt)
	g.ai.Execute()

	g.Redraw(cells, me)

	return true
}

const lineInterval = 100

func (g *game) Redraw(cells []*agario.CellUpdate, me *agario.CellUpdate) {
	g.renderer.SetDrawColor(255, 255, 255, sdl.ALPHA_OPAQUE)
	g.renderer.Clear()

	meHalf := me.Size / 2

	camera := &sdl.Point{
		X: int32((me.X + meHalf/2) - (windowWidth / 2)),
		Y: int32((me.Y + meHalf/2) - (windowHeight / 2)),
	}

	g.renderer.SetDrawColor(230, 230, 230, sdl.ALPHA_OPAQUE)

	g.renderer.DrawRect(&sdl.Rect{
		X: int32(g.Game.Board.Left) - camera.X,
		Y: int32(g.Game.Board.Top) - camera.Y,
		W: int32(g.Game.Board.Right - g.Game.Board.Left),
		H: int32(g.Game.Board.Top - g.Game.Board.Bottom),
	})

	for x := int32(g.Game.Board.Left); x <= int32(g.Game.Board.Right); x += lineInterval {
		points := []sdl.Point{{x - camera.X, int32(g.Game.Board.Top) - camera.Y}, {x - camera.X, int32(g.Game.Board.Bottom) - camera.Y}}
		g.renderer.DrawLines(points)
	}

	for y := int32(g.Game.Board.Top); y <= int32(g.Game.Board.Bottom); y += lineInterval {
		points := []sdl.Point{{int32(g.Game.Board.Left) - camera.X, y - camera.Y}, {int32(g.Game.Board.Right) - camera.X, y - camera.Y}}
		g.renderer.DrawLines(points)
	}

	eatMeSize := int16(float64(me.Size)*1.25) - 1
	canEatSize := int16(float64(me.Size)/1.25) + 1

	for _, cell := range cells {
		drawSize := cell.Size + 40
		halfDrawSize := drawSize / 2

		startX, startY := cell.X-halfDrawSize, cell.Y-halfDrawSize

		_, myCell := g.Game.MyIDs[cell.ID]

		switch {
		case myCell:
			switch g.ai.State {
			case stateFleeing:
				g.renderer.SetDrawColor(232, 118, 0, sdl.ALPHA_OPAQUE)
			case stateHunting:
				g.renderer.SetDrawColor(220, 20, 60, sdl.ALPHA_OPAQUE)
			case stateFeeding:
				g.renderer.SetDrawColor(0, 128, 128, sdl.ALPHA_OPAQUE)
			case stateIdle:
				g.renderer.SetDrawColor(30, 30, 30, sdl.ALPHA_OPAQUE)
			default:
				g.renderer.SetDrawColor(100, 100, 250, sdl.ALPHA_OPAQUE)
			}
		case cell.Virus:
			g.renderer.SetDrawColor(180, 180, 180, sdl.ALPHA_OPAQUE)
		case cell.Size >= eatMeSize:
			g.renderer.SetDrawColor(250, 100, 100, sdl.ALPHA_OPAQUE)
		case cell.Size <= canEatSize:
			g.renderer.SetDrawColor(100, 250, 100, sdl.ALPHA_OPAQUE)
		default:
			//g.renderer.SetDrawColor(cell.R, cell.G, cell.B, sdl.ALPHA_OPAQUE)
			g.renderer.SetDrawColor(160, 82, 45, sdl.ALPHA_OPAQUE)
		}

		rect := &sdl.Rect{
			X: int32(startX) - camera.X,
			Y: int32(startY) - camera.Y,

			W: int32(drawSize),
			H: int32(drawSize),
		}

		g.renderer.FillRect(rect)
	}

	/*if path != nil {
		points := make([]sdl.Point, 1+len(path))

		points[0] = sdl.Point{int32(me.X) - camera.X, int32(me.Y) - camera.Y}

		for i, rawNode := range path {
			node := rawNode.(MapNode)
			points[i+1] = sdl.Point{int32(node.X) - camera.X, int32(node.Y) - camera.Y}
		}

		g.renderer.SetDrawColor(50, 50, 50, sdl.ALPHA_OPAQUE)
		g.renderer.DrawLines(points)
	}*/

	/*if g.NodeList != nil {
		g.renderer.SetDrawColor(255, 0, 0, sdl.ALPHA_OPAQUE)

		for _, rawNode := range g.NodeList {
			node := rawNode.(MapNode)
			if (node.X+1 < camera.MinX || node.X-1 > camera.MaxX) || (node.Y+1 < camera.MinY || node.Y-1 > camera.MaxY) {
				continue
			}
			g.renderer.DrawPoint(int(node.X-camera.MinX), int(node.Y-camera.MinY))
			g.renderer.DrawPoint(int(node.X-camera.MinX)-1, int(node.Y-camera.MinY))
			g.renderer.DrawPoint(int(node.X-camera.MinX)+1, int(node.Y-camera.MinY))
			g.renderer.DrawPoint(int(node.X-camera.MinX), int(node.Y-camera.MinY)-1)
			g.renderer.DrawPoint(int(node.X-camera.MinX), int(node.Y-camera.MinY)+1)
		}
	}*/

	g.renderer.Present()
}

func (g *game) Close() error {
	g.renderer.Destroy()
	g.window.Destroy()
	return nil
}

func (g *game) getOwnCell() *agario.CellUpdate {
	var largest *agario.CellUpdate
	for id := range g.Game.MyIDs {
		cell, ok := g.Game.Cells[id]
		if !ok {
			continue
		}
		if largest == nil || cell.Size > largest.Size || (cell.Size == largest.Size && cell.ID < largest.ID) {
			largest = cell
		}
	}
	return largest
}

type cellArray []*agario.CellUpdate

func (p cellArray) Len() int { return len(p) }
func (p cellArray) Less(i, j int) bool {
	if p[i].Size == p[j].Size {
		return p[i].ID < p[j].ID
	}
	return p[i].Size < p[j].Size
}
func (p cellArray) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (g *game) getSortedCells() []*agario.CellUpdate {
	cells := make([]*agario.CellUpdate, len(g.Game.Cells))

	i := 0
	for _, cell := range g.Game.Cells {
		cells[i] = cell
		i++
	}

	sort.Sort(cellArray(cells))

	return cells
}
