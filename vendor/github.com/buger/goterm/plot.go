package goterm

import (
	"fmt"
	"math"
	"strings"
)

const (
	AXIS_LEFT = iota
	AXIS_RIGHT
)

const (
	DRAW_INDEPENDENT = 1 << iota
	DRAW_RELATIVE
)

type DataTable struct {
	columns []string

	rows [][]float64
}

func (d *DataTable) AddColumn(name string) {
	d.columns = append(d.columns, name)
}

func (d *DataTable) AddRow(elms ...float64) {
	d.rows = append(d.rows, elms)
}

type Chart interface {
	Draw(data DataTable, flags int) string
}

type LineChart struct {
	Buf      []string
	chartBuf []string

	data *DataTable

	Width  int
	Height int

	chartHeight int
	chartWidth  int

	paddingX int

	paddingY int

	Flags int
}

func genBuf(size int) []string {
	buf := make([]string, size)

	for i := 0; i < size; i++ {
		buf[i] = " "
	}

	return buf
}

// Format float
func ff(num interface{}) string {
	return fmt.Sprintf("%.1f", num)
}

func NewLineChart(width, height int) *LineChart {
	chart := new(LineChart)
	chart.Width = width
	chart.Height = height
	chart.Buf = genBuf(width * height)

	// axis lines + axies text
	chart.paddingY = 2

	return chart
}

func (c *LineChart) DrawAxes(maxX, minX, maxY, minY float64, index int) {
	side := AXIS_LEFT

	if c.Flags&DRAW_INDEPENDENT != 0 {
		if index%2 == 0 {
			side = AXIS_RIGHT
		}

		c.DrawLine(c.paddingX-1, 1, c.Width-c.paddingX, 1, "-")
	} else {
		c.DrawLine(c.paddingX-1, 1, c.Width-1, 1, "-")
	}

	if side == AXIS_LEFT {
		c.DrawLine(c.paddingX-1, 1, c.paddingX-1, c.Height-1, "│")
	} else {
		c.DrawLine(c.Width-c.paddingX, 1, c.Width-c.paddingX, c.Height-1, "│")
	}

	left := 0
	if side == AXIS_RIGHT {
		left = c.Width - c.paddingX + 1
	}

	if c.Flags&DRAW_RELATIVE != 0 {
		c.writeText(ff(minY), left, 1)
	} else {
		if minY > 0 {
			c.writeText("0", left, 1)
		} else {
			c.writeText(ff(minY), left, 1)
		}
	}

	c.writeText(ff(maxY), left, c.Height-1)

	c.writeText(ff(minX), c.paddingX, 0)

	x_col := c.data.columns[0]
	c.writeText(c.data.columns[0], c.Width/2-len(x_col)/2, 1)

	if c.Flags&DRAW_INDEPENDENT != 0 || len(c.data.columns) < 3 {
		col := c.data.columns[index]

		for idx, char := range strings.Split(col, "") {
			start_from := c.Height/2 + len(col)/2 - idx

			if side == AXIS_LEFT {
				c.writeText(char, c.paddingX-1, start_from)
			} else {
				c.writeText(char, c.Width-c.paddingX, start_from)
			}
		}
	}

	if c.Flags&DRAW_INDEPENDENT != 0 {
		c.writeText(ff(maxX), c.Width-c.paddingX-len(ff(maxX)), 0)
	} else {
		c.writeText(ff(maxX), c.Width-len(ff(maxX)), 0)
	}
}

func (c *LineChart) writeText(text string, x, y int) {
	coord := y*c.Width + x

	for idx, char := range strings.Split(text, "") {
		c.Buf[coord+idx] = char
	}
}

func (c *LineChart) Draw(data *DataTable) (out string) {
	var scaleY, scaleX float64

	c.data = data

	if c.Flags&DRAW_INDEPENDENT != 0 && len(data.columns) > 3 {
		fmt.Println("Error: Can't use DRAW_INDEPENDENT for more then 2 graphs")
		return ""
	}

	charts := len(data.columns) - 1

	prevPoint := [2]int{-1, -1}

	maxX, minX, maxY, minY := getBoundaryValues(data, -1)

	c.paddingX = int(math.Max(float64(len(ff(minY))), float64(len(ff(maxY))))) + 1

	c.chartHeight = c.Height - c.paddingY

	if c.Flags&DRAW_INDEPENDENT != 0 {
		c.chartWidth = c.Width - 2*c.paddingX
	} else {
		c.chartWidth = c.Width - c.paddingX - 1
	}

	scaleX = float64(c.chartWidth) / (maxX - minX)

	if c.Flags&DRAW_RELATIVE != 0 || minY < 0 {
		scaleY = float64(c.chartHeight) / (maxY - minY)
	} else {
		scaleY = float64(c.chartHeight) / maxY
	}

	for i := 1; i < charts+1; i++ {
		if c.Flags&DRAW_INDEPENDENT != 0 {
			maxX, minX, maxY, minY = getBoundaryValues(data, i)

			scaleX = float64(c.chartWidth-1) / (maxX - minX)
			scaleY = float64(c.chartHeight) / maxY

			if c.Flags&DRAW_RELATIVE != 0 || minY < 0 {
				scaleY = float64(c.chartHeight) / (maxY - minY)
			}
		}

		symbol := Color("•", i)

		c_data := getChartData(data, i)

		for _, point := range c_data {
			x := int((point[0]-minX)*scaleX) + c.paddingX
			y := int((point[1])*scaleY) + c.paddingY

			if c.Flags&DRAW_RELATIVE != 0 || minY < 0 {
				y = int((point[1]-minY)*scaleY) + c.paddingY
			}

			if prevPoint[0] == -1 {
				prevPoint[0] = x
				prevPoint[1] = y
			}

			if prevPoint[0] <= x {
				c.DrawLine(prevPoint[0], prevPoint[1], x, y, symbol)
			}

			prevPoint[0] = x
			prevPoint[1] = y
		}

		c.DrawAxes(maxX, minX, maxY, minY, i)
	}

	for row := c.Height - 1; row >= 0; row-- {
		out += strings.Join(c.Buf[row*c.Width:(row+1)*c.Width], "") + "\n"
	}

	return
}

func (c *LineChart) DrawLine(x0, y0, x1, y1 int, symbol string) {
	drawLine(x0, y0, x1, y1, func(x, y int) {
		coord := y*c.Width + x

		if coord > 0 && coord < len(c.Buf) {
			c.Buf[coord] = symbol
		}
	})
}

func getBoundaryValues(data *DataTable, index int) (maxX, minX, maxY, minY float64) {
	maxX = math.Inf(-1)
	minX = math.Inf(1)
	maxY = math.Inf(-1)
	minY = math.Inf(1)

	for _, r := range data.rows {
		maxX = math.Max(maxX, r[0])
		minX = math.Min(minX, r[0])

		for idx, c := range r {
			if idx > 0 {
				if index == -1 || index == idx {
					maxY = math.Max(maxY, c)
					minY = math.Min(minY, c)
				}
			}
		}
	}

	if maxY > 0 {
		maxY = maxY * 1.1
	} else {
		maxY = maxY * 0.9
	}

	if minY > 0 {
		minY = minY * 0.9
	} else {
		minY = minY * 1.1
	}

	return
}

// DataTable can contain data for multiple graphs, we need to extract only 1
func getChartData(data *DataTable, index int) (out [][]float64) {
	for _, r := range data.rows {
		out = append(out, []float64{r[0], r[index]})
	}

	return
}

// Algorithm for drawing line between two points
//
// http://en.wikipedia.org/wiki/Bresenham's_line_algorithm
func drawLine(x0, y0, x1, y1 int, plot func(int, int)) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	var sx, sy int
	if x0 < x1 {
		sx = 1
	} else {
		sx = -1
	}
	if y0 < y1 {
		sy = 1
	} else {
		sy = -1
	}
	err := dx - dy

	for {
		plot(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}
