package main

import (
	"image/color"
	"log"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/nightexcessive/agario"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/sdl_ttf"
)

const (
	initialWindowWidth  = 1280
	initialWindowHeight = 720
)

func createGame(internalGame *agario.Game) *game {
	var err error

	g := &game{
		Game:         internalGame,
		nicknameSent: time.Time{},
		zoom:         1.0,
	}

	g.ai = &AI{
		g: g,

		Predators: make([]*agario.Cell, 0),
		Prey:      make([]*agario.Cell, 0),
		Food:      make([]*agario.Cell, 0),
	}

	g.window, err = sdl.CreateWindow("agariobot", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, initialWindowWidth, initialWindowHeight, sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE)
	if err != nil {
		panic(err)
	}

	g.renderer, err = sdl.CreateRenderer(g.window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		panic(err)
	}

	g.renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)

	return g
}

type game struct {
	window   *sdl.Window
	renderer *sdl.Renderer

	ai *AI

	zoom float32

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

	cells := g.getSortedCells()

	if len(g.Game.MyIDs) == 0 {
		if cells != nil {
			g.Redraw(cells)
		}
		const (
			idResendTime = 1 * time.Second
			idWaitTime   = 3 * time.Second
		)
		if g.nicknameSent.IsZero() {
			name := randomName()
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
			name := randomName()
			log.Printf("Retrying sending my nickname (%s)...", name)
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

	if !g.ownCellExists() {
		log.Printf("Can't find own cell")
		g.Redraw(cells)
		return true
	}

	g.ai.Update(dt)

	g.Redraw(cells)

	return true
}

const lineInterval = 100

func (g *game) Redraw(cells []*agario.Cell) {
	f, err := ttf.OpenFont("Ubuntu.ttf", int(14.0/g.zoom))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	g.renderer.SetScale(g.zoom, g.zoom)

	if g.ai == nil || g.ai.Me == nil {
		return
	}

	windowWidth, windowHeight := g.window.GetSize()

	g.zoom = calculateZoom(g.ai.Me.Size, windowWidth, windowHeight)

	g.renderer.SetDrawColor(255, 255, 255, sdl.ALPHA_OPAQUE)
	g.renderer.Clear()

	meHalf := g.ai.Me.Size / 2

	meX := int32(g.ai.Me.Position.X())
	meY := int32(g.ai.Me.Position.Y())

	camera := &sdl.Point{
		X: (meX + meHalf/2) - int32(float32(windowWidth)/g.zoom/2),
		Y: (meY + meHalf/2) - int32(float32(windowHeight)/g.zoom/2),
	}

	g.renderer.SetDrawColor(230, 230, 230, sdl.ALPHA_OPAQUE)

	for x := int32(g.Game.Board.Left); x <= int32(g.Game.Board.Right); x += lineInterval {
		points := []sdl.Point{{x - camera.X, int32(g.Game.Board.Top) - camera.Y}, {x - camera.X, int32(g.Game.Board.Bottom) - camera.Y}}
		g.renderer.DrawLines(points)
	}

	for y := int32(g.Game.Board.Top); y <= int32(g.Game.Board.Bottom); y += lineInterval {
		points := []sdl.Point{{int32(g.Game.Board.Left) - camera.X, y - camera.Y}, {int32(g.Game.Board.Right) - camera.X, y - camera.Y}}
		g.renderer.DrawLines(points)
	}

	points := []sdl.Point{{int32(g.Game.Board.Right) - camera.X, int32(g.Game.Board.Top) - camera.Y}, {int32(g.Game.Board.Right) - camera.X, int32(g.Game.Board.Bottom) - camera.Y}}
	g.renderer.DrawLines(points)

	points = []sdl.Point{{int32(g.Game.Board.Left) - camera.X, int32(g.Game.Board.Bottom) - camera.Y}, {int32(g.Game.Board.Right) - camera.X, int32(g.Game.Board.Bottom) - camera.Y}}
	g.renderer.DrawLines(points)

	eatMeSize := int32(float64(g.ai.SmallestOwnCell.Size)*eatSizeRequirement) - 1
	splitEatMeSize := (eatMeSize - 10) * 2
	canEatSize := int32(float64(g.ai.SmallestOwnCell.Size)/eatSizeRequirement) + 1
	canSplitKillSize := canEatSize / 2

	for _, cell := range cells {
		/*drawSize := int32(float32(cell.Size) * 1.35)
		halfDrawSize := drawSize / 2*/

		x, y := int32(cell.Position.X()), int32(cell.Position.Y())

		_, myCell := g.Game.MyIDs[cell.ID]

		switch {
		case myCell:
			switch g.ai.State {
			case stateFleeing:
				g.renderer.SetDrawColor(232, 118, 0, sdl.ALPHA_OPAQUE)
			case stateHunting:
				g.renderer.SetDrawColor(220, 20, 60, sdl.ALPHA_OPAQUE)
			case stateFeeding:
				g.renderer.SetDrawColor(100, 100, 250, sdl.ALPHA_OPAQUE)
			case stateIdle:
				g.renderer.SetDrawColor(100, 100, 100, sdl.ALPHA_OPAQUE)
			default:
				g.renderer.SetDrawColor(0, 0, 0, sdl.ALPHA_OPAQUE)
			}
		case cell.Size <= foodMaxSize: // Food cell
			c := cell.Color.(color.RGBA)
			g.renderer.SetDrawColor(c.R, c.G, c.B, c.A)
		case cell.IsVirus:
			g.renderer.SetDrawColor(51, 255, 51, sdl.ALPHA_OPAQUE)
		case cell.Size >= splitEatMeSize:
			g.renderer.SetDrawColor(0, 0, 0, sdl.ALPHA_OPAQUE)
		case cell.Size >= eatMeSize:
			g.renderer.SetDrawColor(250, 100, 100, sdl.ALPHA_OPAQUE)
		case cell.Size <= canSplitKillSize:
			g.renderer.SetDrawColor(237, 122, 162, sdl.ALPHA_OPAQUE)
		case cell.Size <= canEatSize:
			g.renderer.SetDrawColor(100, 250, 100, sdl.ALPHA_OPAQUE)
		default:
			g.renderer.SetDrawColor(160, 82, 45, sdl.ALPHA_OPAQUE)
		}

		headingLen := float32(cell.Size) * 2
		points := []sdl.Point{{x - camera.X, y - camera.Y}, {x + int32(cell.Heading.X()*headingLen) - camera.X, y + int32(cell.Heading.Y()*headingLen) - camera.Y}}
		g.renderer.DrawLines(points)

		/*rect := &sdl.Rect{
			X: int32(startX) - camera.X,
			Y: int32(startY) - camera.Y,

			W: int32(drawSize),
			H: int32(drawSize),
		}

		g.renderer.FillRect(rect)*/
		fillCircle(g.renderer, &sdl.Point{x - camera.X, y - camera.Y}, int(cell.Size))

		name := cell.Name
		if len(name) > 0 {
			name += " (" + strconv.Itoa(int(cell.Size)) + ")"
			g.renderText(name, f, sdl.Color{0, 0, 0, sdl.ALPHA_OPAQUE}, sdl.Point{x - camera.X, y - camera.Y + cell.Size}, true)
		} else {
			g.renderText("("+strconv.Itoa(int(cell.Size))+")", f, sdl.Color{0, 0, 0, sdl.ALPHA_OPAQUE}, sdl.Point{x - camera.X + cell.Size/2, y - camera.Y + cell.Size}, true)
		}
	}

	if g.ai.Path != nil {
		points := make([]sdl.Point, 0, len(g.ai.Path))
		for _, vec := range g.ai.Path {
			points = append(points, sdl.Point{int32(vec[0]) - camera.X, int32(vec[1]) - camera.Y})
		}

		g.renderer.SetDrawColor(50, 50, 50, sdl.ALPHA_OPAQUE)
		g.renderer.DrawLines(points)

		g.renderer.SetDrawColor(0, 100, 0, sdl.ALPHA_OPAQUE)
		for _, vec := range g.ai.Path {
			x, y := int(vec[0])-int(camera.X), int(vec[1])-int(camera.Y)
			g.renderer.DrawLine(x-costMapReduction/4, y, x+costMapReduction/4, y)
			g.renderer.DrawLine(x, y-costMapReduction/4, x, y+costMapReduction/4)
		}
	}

	for x, rows := range g.ai.Map {
		for y, v := range rows {
			g.renderer.SetDrawColor(255, 0, 0, uint8(255*(v/costDoNotPass)))

			/*g.renderer.FillRect(&sdl.Rect{
				X: int32(x*costMapReduction) - camera.X,
				Y: int32(y*costMapReduction) - camera.Y,

				W: costMapReduction,
				H: costMapReduction,
			})*/

			screenX := int(int32(x*costMapReduction) - camera.X)
			screenY := int(int32(y*costMapReduction) - camera.Y)

			g.renderer.DrawLine(screenX, screenY-costMapReduction/2, screenX, screenY+costMapReduction/2)
			g.renderer.DrawLine(screenX-costMapReduction/2, screenY, screenX+costMapReduction/2, screenY)
		}
	}

	leaderboardY := int32(0)
	for i, item := range g.Game.Leaderboard {
		name := item.Name
		if name == "" {
			name = "an unnamed cell"
		}
		c := sdl.Color{0, 0, 0, sdl.ALPHA_OPAQUE}
		if _, myCell := g.Game.MyIDs[item.ID]; myCell {
			c = sdl.Color{250, 100, 100, sdl.ALPHA_OPAQUE}
		}
		_, h := g.renderText(strconv.Itoa(i+1)+". "+name, f, c, sdl.Point{0, leaderboardY}, false)
		leaderboardY += h + 3
	}

	statusY := int32(0)
	for _, msg := range g.ai.Status {
		_, h := g.renderText(msg, f, sdl.Color{0, 0, 0, sdl.ALPHA_OPAQUE}, sdl.Point{int32(float32(windowWidth) / g.zoom / 2), statusY}, true)
		statusY += h + 3
	}

	g.renderer.Present()
}

func (g *game) renderText(str string, f *ttf.Font, color sdl.Color, p sdl.Point, center bool) (w, h int32) {
	fw, fh, _ := f.SizeUTF8(str)
	w, h = int32(fw), int32(fh)
	fontRendered, _ := f.RenderUTF8_Solid(str, color)
	defer fontRendered.Free()
	t, _ := g.renderer.CreateTextureFromSurface(fontRendered)
	defer t.Destroy()
	var drawRect *sdl.Rect
	if !center {
		drawRect = &sdl.Rect{p.X, p.Y, w, h}
	} else {
		drawRect = &sdl.Rect{p.X - w/2, p.Y, w, h}
	}
	g.renderer.Copy(t, nil, drawRect)
	return
}

func (g *game) Close() error {
	g.renderer.Destroy()
	g.window.Destroy()
	return nil
}

func (g *game) ownCellExists() bool {
	for id := range g.Game.MyIDs {
		_, ok := g.Game.Cells[id]
		if !ok {
			continue
		}
		return true
	}
	return false
}

type cellArray []*agario.Cell

func (p cellArray) Len() int { return len(p) }
func (p cellArray) Less(i, j int) bool {
	if p[i].Size == p[j].Size {
		return p[i].ID < p[j].ID
	}
	return p[i].Size < p[j].Size
}
func (p cellArray) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (g *game) getSortedCells() []*agario.Cell {
	cells := make([]*agario.Cell, len(g.Game.Cells))

	i := 0
	for _, cell := range g.Game.Cells {
		cells[i] = cell
		i++
	}

	sort.Sort(cellArray(cells))

	return cells
}

func calculateZoom(totalSize int32, windowWidth, windowHeight int) float32 {
	f := math.Pow(math.Min(64.0/float64(totalSize), 1.0), 1.0/2.5)
	d := f * math.Max(float64(windowHeight)/1080.0, float64(windowWidth)/1920.0)

	return float32(d)
}

func fillCircle(s *sdl.Renderer, p *sdl.Point, radius int) {
	pX, pY := int(p.X), int(p.Y)
	x := radius
	y := 0
	decisionOver2 := 1 - x

	for x >= y {
		s.DrawLine(x+pX, y+pY, -x+pX, y+pY)
		s.DrawLine(x+pX, -y+pY, -x+pX, -y+pY)

		s.DrawLine(y+pX, x+pY, -y+pX, x+pY)
		s.DrawLine(y+pX, -x+pY, -y+pX, -x+pY)

		y++
		if decisionOver2 <= 0 {
			decisionOver2 += 2*y + 1
		} else {
			x--
			decisionOver2 += 2*(y-x) + 1
		}
	}
}
