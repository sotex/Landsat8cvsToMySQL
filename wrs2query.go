// WRS2Query project main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/geojson"

	"github.com/jonas-p/go-shp"
	geometry "github.com/tidwall/geojson/geometry"
)

type (
	WRS2Object struct {
		Path     int            // 路径
		Row      int            // 行
		boundary *geometry.Poly // 有效范围
	}
)

// 转换为 geojson 格式
func (obj *WRS2Object) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(`{"type":"Feature","properties":{"path":`)
	buf.WriteString(strconv.Itoa(obj.Path))
	buf.WriteString(`,"row":`)
	buf.WriteString(strconv.Itoa(obj.Row))
	buf.WriteString(`},"geometry":`)
	data, err := json.Marshal(geojson.NewPolygon(obj.boundary))
	if err != nil {
		return nil, err
	}
	buf.Write(data)
	buf.WriteByte('}')
	return buf.Bytes(), err
}

var (
	// WRS2索引 key=path*1000+row
	sPathRowIndex map[int]*WRS2Object
)

func init() {
	sPathRowIndex = make(map[int]*WRS2Object)
}

// 加载 wrs2 数据
// 这里加载的数据是经过处理的，把极地附近的给去掉了
func loadWRS2Data(shpfile string) error {
	shape, err := shp.Open(shpfile)
	if err != nil {
		return fmt.Errorf("打开文件错误:%s", err.Error())
	}
	defer shape.Close()

	// 字段信息
	// AREA PERIMETER PR_ PR_ID RINGS_OK RINGS_NOK PATH ROW MODE SEQUENCE WRSPR PR ACQDayL7 ACQDayL8
	// fields := shape.Fields()
	// fmt.Println(fields)
	for shape.Next() {
		n, s := shape.Shape()
		p, ok := s.(*shp.Polygon)
		if !ok {
			continue
		}
		// 获取边界点坐标
		coordinates := make([][]geometry.Point, p.NumParts)
		for i := int32(0); i < p.NumParts; i += 1 {
			var startIndex, endIndex int32
			startIndex = p.Parts[i]
			if i == p.NumParts-1 {
				endIndex = int32(len(p.Points))
			} else {
				endIndex = p.Parts[i+1]
			}

			coordinates[i] = make([]geometry.Point, 0, endIndex-startIndex)
			cnt := 0
			for j := startIndex; j < endIndex; j = j + 1 {
				var point = geometry.Point{
					X: p.Points[j].X,
					Y: p.Points[j].Y,
				}
				// 如果不是第一个或者最后一个点，判断与前一个点的距离
				// 用于减少点数（变成四边形，虽然不准确）
				if cnt > 0 && cnt < int(endIndex-startIndex-1) {
					dx, dy := point.X-coordinates[i][cnt-1].X, point.Y-coordinates[i][cnt-1].Y
					// 如果距离小于0.1度，就认为是一个点
					if (dx*dx + dy*dy) < 0.1 {
						continue
					}
				}
				coordinates[i] = append(coordinates[i], point)
				cnt = cnt + 1
			}
		}
		// 获取路径和行号
		var path, row int
		path, _ = strconv.Atoi(shape.ReadAttribute(n, 6))
		row, _ = strconv.Atoi(shape.ReadAttribute(n, 7))
		// 插入map里面
		var pr = path*1000 + row
		polygon := geometry.NewPoly(coordinates[0], coordinates[1:], nil)
		sPathRowIndex[pr] = &WRS2Object{
			Path:     path,
			Row:      row,
			boundary: polygon,
		}
	}
	return nil
}

func queryWRS2(path, row int) *WRS2Object {
	p, ok := sPathRowIndex[path*1000+row]
	if ok {
		return p
	}
	return nil
}

func polygonToWkt(polygon *geometry.Poly) (string, error) {
	if polygon.Empty() {
		return "", fmt.Errorf("polygon is empty")
	}
	var builder = &strings.Builder{}
	builder.WriteString("POLYGON((")

	exterior := polygon.Exterior
	numpoints := exterior.NumPoints()
	for i := 0; i < numpoints-1; i++ {
		var point = exterior.PointAt(i)
		builder.WriteString(strconv.FormatFloat(point.X, 'f', -1, 64))
		builder.WriteByte(' ')
		builder.WriteString(strconv.FormatFloat(point.Y, 'f', -1, 64))
		builder.WriteByte(',')
	}
	var point = exterior.PointAt(0)
	builder.WriteString(strconv.FormatFloat(point.X, 'f', -1, 64))
	builder.WriteByte(' ')
	builder.WriteString(strconv.FormatFloat(point.Y, 'f', -1, 64))

	builder.WriteString("))")
	return builder.String(), nil
}
