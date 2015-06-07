package main

import (
	"math"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/search"
	"github.com/nightexcessive/agario"
)

const boardReductionFactor = 100

func findNearestFood(g *game, cells []*agario.CellUpdate, me *agario.CellUpdate, from graph.Node) []graph.Node {
	//paths, costs := search.Dijkstra(from, g.Map, nil)

	canEatSize := int16(float64(me.Size)/1.25) - 1
	minimumSize := int16(float64(me.Size) / 10)
	if minimumSize < 100 {
		minimumSize = 100
	}

	lowestCost := math.MaxFloat64
	var lowestCostPath []graph.Node
	for _, test := range cells {
		if test.Virus {
			continue
		}

		if _, ok := g.Game.MyIDs[test.ID]; ok {
			continue
		}

		if test.Size < minimumSize || test.Size > canEatSize {
			//log.Printf("Not chasing %q (%d): Not edible (%d <= %d)", test.Name, test.ID, test.Size, me.Size)
			continue
		}

		//id := MapNode{g.Game, test.Point}.ID()
		cost := dist2(from.(MapNode).Point, test.Point)
		if cost < lowestCost {
			lowestCost = cost
			lowestCostPath = []graph.Node{MapNode{g.Game, test.Point}}
		}
	}

	return lowestCostPath
}

func findNearestVirus(g *game, cells []*agario.CellUpdate, me *agario.CellUpdate, from graph.Node) []graph.Node {
	//paths, costs := search.Dijkstra(from, g.Map, nil)

	popSize := int16(float64(me.Size) / 1.25)

	lowestCost := math.MaxFloat64
	var lowestCostPath []graph.Node
	for _, test := range cells {
		if !test.Virus {
			continue
		}

		if test.Size <= maxSize {
			continue
		}

		cost := dist2(from.(MapNode).Point, test.Point)
		if cost < lowestCost {
			lowestCost = cost
			lowestCostPath = []graph.Node{MapNode{g.Game, test.Point}}
		}
	}

	return lowestCostPath
}

func findPath(g graph.Graph, from, to graph.Node) []graph.Node {
	path, _, _ := search.AStar(from, to, g, nil, nil)

	return path
}

func createMap(g *agario.Game) *Map {
	m := &Map{
		G: g,
	}
	m.MinNode = serializePosition(g, int16(g.Board.Left), int16(g.Board.Top)).ID()
	m.MaxNode = serializePosition(g, int16(g.Board.Right), int16(g.Board.Bottom)).ID()

	return m
}

type Map struct {
	G *agario.Game

	MinNode int
	MaxNode int
}

func (m *Map) EdgeBetween(rawNode, rawNeighbor graph.Node) graph.Edge {
	node, ok := rawNode.(MapNode)
	if !ok {
		node = unserializePosition(m.G, rawNode.ID())
	}
	neighbor, ok := rawNeighbor.(MapNode)
	if !ok {
		neighbor = unserializePosition(m.G, rawNeighbor.ID())
	}

	xDist := node.X - neighbor.X
	if xDist < 0 {
		xDist = -xDist
	}
	yDist := node.Y - neighbor.Y
	if yDist < 0 {
		yDist = -yDist
	}

	if xDist <= boardReductionFactor && yDist <= boardReductionFactor {
		return concrete.Edge{rawNode, rawNeighbor}
	}

	return nil
}

func (m *Map) Neighbors(rawNode graph.Node) []graph.Node {
	node, ok := rawNode.(MapNode)
	if !ok {
		node = unserializePosition(m.G, rawNode.ID())
	}

	neighbors := make([]graph.Node, 0, 9)

	startX := roundToReduction(node.X)
	startY := roundToReduction(node.Y)

	for x := startX - boardReductionFactor; x <= startX+boardReductionFactor; x += boardReductionFactor {
		for y := startY - boardReductionFactor; y <= startY+boardReductionFactor; y += boardReductionFactor {
			if x == node.X && y == node.Y {
				continue
			}

			neighbors = append(neighbors, serializePosition(m.G, x, y))
		}
	}

	return neighbors
}

func (m *Map) NodeExists(node graph.Node) bool {
	id := node.ID()

	return id >= m.MinNode && id <= m.MaxNode
}

func (m *Map) NodeList() []graph.Node {
	var (
		w = int16(m.G.Board.Right) / boardReductionFactor
		h = int16(m.G.Board.Bottom) / boardReductionFactor
	)

	nodes := make([]graph.Node, 0, w*h)

	for x := int16(0); x <= w*boardReductionFactor; x += boardReductionFactor {
		for y := int16(0); y <= h*boardReductionFactor; y += boardReductionFactor {
			nodes = append(nodes, serializePosition(m.G, x, y))
		}
	}

	//log.Printf("nodes: %d (%d)", len(nodes), cap(nodes))

	return nodes
}

type MapNode struct {
	G *agario.Game
	agario.Point
}

func (n MapNode) ID() int {
	return int(n.X + int16(n.G.Board.Right)*n.Y)
}

func serializePosition(g *agario.Game, x, y int16) MapNode {
	return MapNode{g, agario.Point{x, y}}
}

func unserializePosition(g *agario.Game, id int) MapNode {
	x := math.Mod(float64(id), float64(g.Board.Right))
	y := id / int(g.Board.Right)
	return MapNode{g, agario.Point{int16(x), int16(y)}}
}

func roundToReduction(x int16) int16 {
	return x / boardReductionFactor * boardReductionFactor
}

func roundPointToReduction(x MapNode) MapNode {
	return MapNode{
		G: x.G,

		Point: agario.Point{
			X: roundToReduction(x.X),
			Y: roundToReduction(x.Y),
		},
	}
}

func square(a float64) float64 {
	return a * a
}

func dist2(a, b agario.Point) float64 {
	x1, y1 := float64(a.X), float64(a.Y)
	x2, y2 := float64(b.X), float64(b.Y)

	return square(x2-x1) + square(y2-y1)
}
