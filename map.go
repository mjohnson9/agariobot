package main

import (
	"log"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

var (
	_ graph.DirectedGraph   = NewMap(10, 10)
	_ graph.Graph           = NewMap(10, 10)
	_ graph.Coster          = NewMap(10, 10)
	_ graph.HeuristicCoster = NewMap(10, 10)
)

var (
	_ graph.Graph           = UndirectedMap(NewMap(10, 10))
	_ graph.Coster          = UndirectedMap(NewMap(10, 10))
	_ graph.HeuristicCoster = UndirectedMap(NewMap(10, 10))
)

func NewMap(w, h int) Map {
	return Map(make2DMap(w, h))
}

type Map [][]float32

func (m Map) SetCellCost(x, y int, cost float32) {
	m[x][y] = cost
}

func (m Map) AddCellCost(x, y int, cost float32) {
	m[x][y] += cost
}

func (m Map) GetCellCost(x, y int) float32 {
	return m[x][y]
}

func (m Map) NodeList() []graph.Node {
	//panic("Do not attempt to use NodeList - it's too slow")
	w, h := m.width(), m.height()

	nodes := make([]graph.Node, 0, w*h)

	for x, column := range m {
		for y, v := range column {
			nodes = append(nodes, &mapNode{
				W: w,
				H: h,

				X: x,
				Y: y,

				Value: v,
			})
		}
	}

	return nodes
}

func (m Map) NodeExists(nRaw graph.Node) bool {
	var (
		x int
		y int

		w = m.width()
		h = m.height()
	)
	n := nRaw.(*mapNode)
	x, y = n.X, n.Y

	return x >= 0 && x < w && y >= 0 && y < h
}

func (m Map) Predecessors(nRaw graph.Node) []graph.Node {
	n := nRaw.(*mapNode)
	if n.Value >= costDoNotPass {
		return []graph.Node{}
	}

	return m.Successors(n)
}

func (m Map) Successors(nRaw graph.Node) []graph.Node {
	return m.neighbors(nRaw, true)
}

func (m Map) neighbors(nRaw graph.Node, directed bool) []graph.Node {
	var (
		x int
		y int

		w = m.width()
		h = m.height()
	)
	n := nRaw.(*mapNode)
	x, y = n.X, n.Y

	if x < 0 || x >= w || y < 0 || y >= h {
		return nil
	}

	startX := x - 1
	if startX < 0 {
		startX = 0
	}
	startY := y - 1
	if startY < 0 {
		startY = 0
	}

	endX := startX + 2
	if endX >= w {
		endX = w - 1
	}
	endY := startY + 2
	if endY >= h {
		endY = h - 1
	}

	neighbors := make([]graph.Node, 0, 8)
	for nX := startX; nX <= endX; nX++ {
		for nY := startY; nY <= endY; nY++ {
			if nX == x && nY == y {
				continue
			}

			v := m[nX][nY]

			if directed && v >= costDoNotPass {
				continue
			}

			neighbors = append(neighbors, &mapNode{
				W: w,
				H: h,

				X: nX,
				Y: nY,

				Value: v,
			})
		}
	}

	if len(neighbors) > 8 {
		log.Printf("WARNING: neighbors > 8: %+v", neighbors)
	}

	return neighbors
}

func (m Map) EdgeBetween(fRaw, tRaw graph.Node) graph.Edge {
	w, h := m.width(), m.height()

	f := fRaw.(*mapNode)
	x1, y1 := f.X, f.Y

	if x1 < 0 || x1 >= w || y1 < 0 || y1 >= h {
		return nil
	}

	t := tRaw.(*mapNode)
	x2, y2 := t.X, t.Y

	if x2 < 0 || x2 >= w || y2 < 0 || y2 >= h {
		return nil
	}

	return &concrete.Edge{
		F: f,
		T: t,
	}
}

func (m Map) EdgeTo(fRaw, tRaw graph.Node) graph.Edge {
	t := tRaw.(*mapNode)

	if t.Value >= costDoNotPass {
		return nil
	}

	return m.EdgeBetween(fRaw, tRaw)
}

func (m Map) Cost(eRaw graph.Edge) float64 {
	e := eRaw.(*concrete.Edge)

	f := e.F.(*mapNode)
	t := e.T.(*mapNode)

	c := float64(square(float32(t.X-f.X)) + square(float32(t.Y-f.Y)))

	return c
}

func (m Map) HeuristicCost(fRaw, tRaw graph.Node) float64 {
	//f := fRaw.(*mapNode)
	t := tRaw.(*mapNode)

	return float64(t.Value)
}

func (m Map) GetNode(x, y int) graph.Node {
	w, h := m.width(), m.height()
	if x < 0 || y < 0 || x >= w || y >= h {
		return nil
	}

	return &mapNode{
		W: w,
		H: h,

		X: x,
		Y: y,

		Value: m[x][y],
	}
}

func (m Map) width() int {
	return len(m)
}

func (m Map) height() int {
	return len(m[0])
}

func (m Map) Neighbors(nRaw graph.Node) []graph.Node {
	return m.neighbors(nRaw, false)
}

/*func (m Map) quickDijkstra(from graph.Node, match func(graph.Node) bool) []graph.Node {
	return
}*/

type UndirectedMap Map

func (m UndirectedMap) NodeExists(n graph.Node) bool {
	return Map(m).NodeExists(n)
}

func (m UndirectedMap) NodeList() []graph.Node {
	return Map(m).NodeList()
}

func (m UndirectedMap) Neighbors(n graph.Node) []graph.Node {
	return Map(m).Neighbors(n)
}

func (m UndirectedMap) EdgeBetween(node, neighbor graph.Node) graph.Edge {
	return Map(m).EdgeBetween(node, neighbor)
}

func (m UndirectedMap) HeuristicCost(n1, n2 graph.Node) float64 {
	return Map(m).HeuristicCost(n1, n2)
}

func (m UndirectedMap) Cost(e graph.Edge) float64 {
	return Map(m).Cost(e)
}

func nodeCoordsFromID(id, w, h int) (int, int) {
	x := id / w
	y := id - (id * w)

	return x, y
}

type mapNode struct {
	W, H int

	X, Y int

	Value float32
}

func (n *mapNode) ID() int {
	return n.X*n.W + n.Y
}

func make2DMap(w, h int) [][]float32 {
	columns := make([][]float32, w)
	internalMap := make([]float32, w*h)

	for x := 0; x < w; x++ {
		columns[x], internalMap = internalMap[:h], internalMap[h:]
	}

	return columns
}
