package geojson

import (
	"github.com/tidwall/geojson/geometry"
	"github.com/tidwall/gjson"
)

// Polygon ...
type Polygon struct {
	base  geometry.Poly
	extra *extra
}

// Empty ...
func (g *Polygon) Empty() bool {
	if g.extra != nil && g.extra.bbox != nil {
		return false
	}
	return g.base.Empty()
}

// Rect ...
func (g *Polygon) Rect() geometry.Rect {
	if g.extra != nil && g.extra.bbox != nil {
		return *g.extra.bbox
	}
	return g.base.Rect()
}

// Center ...
func (g *Polygon) Center() geometry.Point {
	return g.Rect().Center()
}

// AppendJSON ...
func (g *Polygon) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"type":"Polygon","coordinates":[`...)
	var pidx int
	dst, pidx = appendJSONSeries(dst, g.base.Exterior, g.extra, pidx)
	for _, hole := range g.base.Holes {
		dst = append(dst, ',')
		dst, pidx = appendJSONSeries(dst, hole, g.extra, pidx)
	}
	dst = append(dst, ']')
	if g.extra != nil {
		dst = g.extra.appendJSONExtra(dst)
	}
	dst = append(dst, '}')
	return dst
}

// String ...
func (g *Polygon) String() string {
	return string(g.AppendJSON(nil))
}

// forEach ...
func (g *Polygon) forEach(iter func(geom Object) bool) bool {
	return iter(g)
}

// Within ...
func (g *Polygon) Within(obj Object) bool {
	return obj.Contains(g)
}

// Contains ...
func (g *Polygon) Contains(obj Object) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return obj.withinRect(*g.extra.bbox)
	}
	return obj.withinPoly(&g.base)
}

func (g *Polygon) withinRect(rect geometry.Rect) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return rect.ContainsRect(*g.extra.bbox)
	}
	return rect.ContainsPoly(&g.base)
}

func (g *Polygon) withinPoint(point geometry.Point) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return point.ContainsRect(*g.extra.bbox)
	}
	return point.ContainsPoly(&g.base)
}

func (g *Polygon) withinLine(line *geometry.Line) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return line.ContainsRect(*g.extra.bbox)
	}
	return line.ContainsPoly(&g.base)
}

func (g *Polygon) withinPoly(poly *geometry.Poly) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return poly.ContainsRect(*g.extra.bbox)
	}
	return poly.ContainsPoly(&g.base)
}

// Intersects ...
func (g *Polygon) Intersects(obj Object) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return obj.intersectsRect(*g.extra.bbox)
	}
	return obj.intersectsPoly(&g.base)
}

func (g *Polygon) intersectsPoint(point geometry.Point) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return g.extra.bbox.IntersectsPoint(point)
	}
	return g.base.IntersectsPoint(point)
}

func (g *Polygon) intersectsRect(rect geometry.Rect) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return g.extra.bbox.IntersectsRect(rect)
	}
	return g.base.IntersectsRect(rect)
}

func (g *Polygon) intersectsLine(line *geometry.Line) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return g.extra.bbox.IntersectsLine(line)
	}
	return g.base.IntersectsLine(line)
}

func (g *Polygon) intersectsPoly(poly *geometry.Poly) bool {
	if g.extra != nil && g.extra.bbox != nil {
		return g.extra.bbox.IntersectsPoly(poly)
	}
	return g.base.IntersectsPoly(poly)
}

// NumPoints ...
func (g *Polygon) NumPoints() int {
	n := g.base.Exterior.NumPoints()
	for _, hole := range g.base.Holes {
		n += hole.NumPoints()
	}
	return n
}

func parseJSONPolygon(keys *parseKeys, opts *ParseOptions) (Object, error) {
	var g Polygon
	coords, ex, err := parseJSONPolygonCoords(keys, gjson.Result{}, opts)
	if err != nil {
		return nil, err
	}
	if len(coords) == 0 {
		return nil, errCoordinatesInvalid // must be a linear ring
	}
	for _, p := range coords {
		if len(p) < 4 || p[0] != p[len(p)-1] {
			return nil, errCoordinatesInvalid // must be a linear ring
		}
	}
	exterior := coords[0]
	var holes [][]geometry.Point
	if len(coords) > 1 {
		holes = coords[1:]
	}
	poly := geometry.NewPoly(exterior, holes, opts.IndexGeometry)
	g.base = *poly
	g.extra = ex
	if err := parseBBoxAndExtras(&g.extra, keys, opts); err != nil {
		return nil, err
	}
	return &g, nil
}

func parseJSONPolygonCoords(
	keys *parseKeys, rcoords gjson.Result, opts *ParseOptions,
) (
	[][]geometry.Point, *extra, error,
) {
	var err error
	var coords [][]geometry.Point
	var ex *extra
	var dims int
	if !rcoords.Exists() {
		rcoords = keys.rCoordinates
		if !rcoords.Exists() {
			return nil, nil, errCoordinatesMissing
		}
		if !rcoords.IsArray() {
			return nil, nil, errCoordinatesInvalid
		}
	}
	rcoords.ForEach(func(key, value gjson.Result) bool {
		if !value.IsArray() {
			err = errCoordinatesInvalid
			return false
		}
		coords = append(coords, []geometry.Point{})
		ii := len(coords) - 1
		value.ForEach(func(key, value gjson.Result) bool {
			var count int
			var nums [4]float64
			value.ForEach(func(key, value gjson.Result) bool {
				if count == 4 {
					return false
				}
				if value.Type != gjson.Number {
					err = errCoordinatesInvalid
					return false
				}
				nums[count] = value.Float()
				count++
				return true
			})
			if err != nil {
				return false
			}
			if count < 2 {
				err = errCoordinatesInvalid
				return false
			}
			coords[ii] = append(coords[ii], geometry.Point{X: nums[0], Y: nums[1]})
			if ex == nil {
				if count > 2 {
					ex = new(extra)
					if count > 3 {
						ex.dims = 2
					} else {
						ex.dims = 1
					}
					dims = int(ex.dims)
				}
			}
			if ex != nil {
				for i := 0; i < dims; i++ {
					ex.values = append(ex.values, nums[2+i])
				}
			}
			return true
		})
		return err == nil
	})
	if err != nil {
		return nil, nil, err
	}
	return coords, ex, err
}
