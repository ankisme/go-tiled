/*
Copyright (c) 2017 Lauris Bukšis-Haberkorns <lauris@nix.lv>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package tiled

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"path/filepath"
)

const (
	tileHorizontalFlipMask = 0x80000000
	tileVerticalFlipMask   = 0x40000000
	tileDiagonalFlipMask   = 0x20000000
	tileFlip               = tileHorizontalFlipMask | tileVerticalFlipMask | tileDiagonalFlipMask
	tileGIDMask            = 0x0fffffff
)

// ErrInvalidTileGID error is returned when tile GID is not found
var ErrInvalidTileGID = errors.New("tiled: invalid tile GID")

// Axis type
type Axis string

const (
	// AxisX is X axis
	AxisX Axis = "x"
	// AxisY is Y axis
	AxisY Axis = "y"
)

// StaggerIndexType is stagger axis index type
type StaggerIndexType string

const (
	// StaggerIndexOdd is odd stagger index
	StaggerIndexOdd StaggerIndexType = "odd"
	// StaggerIndexEven is even stagger index
	StaggerIndexEven StaggerIndexType = "even"
)

type Border struct {
	MinX   int
	MinY   int
	MaxX   int
	MaxY   int
	Width  int
	Height int
	Square int
}

// Map contains three different kinds of layers.
// Tile layers were once the only type, and are simply called layer, object layers have the objectgroup tag
// and image layers use the imagelayer tag. The order in which these layers appear is the order in which the
// layers are rendered by Tiled
type Map struct {
	// Loader for loading additional data
	loader *loader `xml:"-"`
	// Base directory for loading additional data
	baseDir string

	// The TMX format version, generally 1.0.
	Version string `xml:"version,attr"`
	// The Tiled version used to generate this file
	TiledVersion string `xml:"tiledversion,attr"`
	// The class of this map (since 1.9, defaults to "").
	Class string `xml:"class,attr"`
	// Map orientation. Tiled supports "orthogonal", "isometric", "staggered" (since 0.9) and "hexagonal" (since 0.11).
	Orientation string `xml:"orientation,attr"`
	// The order in which tiles on tile layers are rendered. Valid values are right-down (the default), right-up, left-down and left-up.
	// In all cases, the map is drawn row-by-row. (since 0.10, but only supported for orthogonal maps at the moment)
	RenderOrder string `xml:"renderorder,attr"`
	// The map width in tiles.
	Width int `xml:"width,attr"`
	// The map height in tiles.
	Height int `xml:"height,attr"`
	// The width of a tile.
	TileWidth int `xml:"tilewidth,attr"`
	// The height of a tile.
	TileHeight int `xml:"tileheight,attr"`
	// Only for hexagonal maps. Determines the width or height (depending on the staggered axis) of the tile's edge, in pixels.
	HexSideLength int `xml:"hexsidelength,attr"`
	// For staggered and hexagonal maps, determines which axis ("x" or "y") is staggered. (since 0.11)
	StaggerAxis Axis `xml:"staggeraxis,attr"`
	// For staggered and hexagonal maps, determines whether the "even" or "odd" indexes along the staggered axis are shifted. (since 0.11)
	StaggerIndex StaggerIndexType `xml:"staggerindex,attr"`
	// The background color of the map. (since 0.9, optional, may include alpha value since 0.15 in the form #AARRGGBB)
	BackgroundColor *HexColor `xml:"backgroundcolor,attr"`
	// Stores the next available ID for new objects. This number is stored to prevent reuse of the same ID after objects have been removed. (since 0.11)
	NextObjectID uint32 `xml:"nextobjectid,attr"`
	IsInfinite   bool   `xml:"infinite,attr"`
	// Custom properties
	Properties *Properties `xml:"properties>property"`
	// Map tilesets
	Tilesets []*Tileset `xml:"tileset"`
	// Map layers
	Layers []*Layer `xml:"layer"`
	// Map object groups
	ObjectGroups []*ObjectGroup `xml:"objectgroup"`
	// Image layers
	ImageLayers []*ImageLayer `xml:"imagelayer"`
	// Group layers
	Groups []*Group `xml:"group"`

	AllLayers []*Layer
	Border    *Border
}

func (m *Map) initTileset(ts *Tileset) error {
	if ts.SourceLoaded {
		return nil
	}
	if len(ts.Source) == 0 {
		ts.baseDir = m.baseDir
		ts.SourceLoaded = true
		return nil
	}
	sourcePath := m.GetFileFullPath(ts.Source)
	f, err := m.loader.open(sourcePath)
	if err != nil {
		return err
	}
	defer f.Close()

	d := xml.NewDecoder(f)

	if err := d.Decode(ts); err != nil {
		return err
	}

	ts.baseDir = filepath.Dir(sourcePath)
	ts.SourceLoaded = true

	return nil
}

// TileGIDToTile is used to find tile data by GID
func (m *Map) TileGIDToTile(gid uint32) (*LayerTile, error) {
	if gid == 0 {
		return NilLayerTile, nil
	}

	gidBare := gid &^ tileFlip

	for i := len(m.Tilesets) - 1; i >= 0; i-- {
		if m.Tilesets[i].FirstGID <= gidBare {
			ts := m.Tilesets[i]
			if err := m.initTileset(ts); err != nil {
				return nil, err
			}
			return &LayerTile{
				ID:             gidBare - ts.FirstGID,
				Tileset:        ts,
				HorizontalFlip: gid&tileHorizontalFlipMask != 0,
				VerticalFlip:   gid&tileVerticalFlipMask != 0,
				DiagonalFlip:   gid&tileDiagonalFlipMask != 0,
				Nil:            false,
			}, nil
		}
	}

	return nil, ErrInvalidTileGID
}

// GetFileFullPath returns path to file relative to map file
func (m *Map) GetFileFullPath(fileName string) string {
	return filepath.Join(m.baseDir, fileName)
}

func (m *Map) RefreshMapWidthInInfiniteMode() {
	minX := 0
	maxX := 0
	minY := 0
	maxY := 0
	hasValue := false

	for _, layer := range m.AllLayers {
		for _, chunk := range layer.Chunks {
			bigX := chunk.X + chunk.Width - 1
			bigY := chunk.Y + chunk.Height - 1

			if hasValue {
				if chunk.X < minX {
					minX = chunk.X
				}

				if bigX > maxX {
					maxX = bigX
				}

				if chunk.Y < minY {
					minY = chunk.Y
				}

				if bigY > maxY {
					maxY = bigY
				}
			} else {
				minX = chunk.X
				maxX = bigX
				minY = chunk.Y
				maxY = bigY
				hasValue = true
			}
		}
	}

	m.Width = maxX - minX + 1
	m.Height = maxY - minY + 1

	border := &Border{
		MinX:   minX,
		MinY:   minY,
		MaxX:   maxX,
		MaxY:   maxY,
		Width:  m.Width,
		Height: m.Height,
	}
	border.Square = border.Width * border.Height
	m.Border = border
}

// UnmarshalXML decodes a single XML element beginning with the given start element.
func (m *Map) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	item := aliasMap{
		loader:  m.loader,
		baseDir: m.baseDir,
	}
	item.SetDefaults()

	if err := d.DecodeElement(&item, &start); err != nil {
		return err
	}

	// Decode Groups data
	for i := 0; i < len(item.Groups); i++ {
		g := item.Groups[i]
		if err := g.DecodeGroup((*Map)(&item)); err != nil {
			return err
		}
	}

	// Decode layers data
	for i := 0; i < len(item.Layers); i++ {
		l := item.Layers[i]
		if err := l.DecodeLayer((*Map)(&item)); err != nil {
			return err
		}
	}

	// Decode object groups.
	for _, g := range item.ObjectGroups {
		if err := g.DecodeObjectGroup((*Map)(&item)); err != nil {
			return err
		}
	}

	allLayers := append([]*Layer{}, item.Layers...)

	for _, group := range item.Groups {
		allLayers = append(allLayers, group.Layers...)
	}

	item.AllLayers = allLayers

	if item.IsInfinite {
		castMap := (*Map)(&item)
		castMap.RefreshMapWidthInInfiniteMode()

		for _, layer := range item.AllLayers {
			layer.ParseLayerInInfiniteMode(castMap)
		}
	}

	*m = (Map)(item)
	return nil
}

func (m *Map) GetLayerByName(name string) *Layer {
	for _, layer := range m.AllLayers {
		if layer.Name == name {
			return layer
		}
	}

	return nil
}

func (m *Map) MustGetLayerByName(name string) *Layer {
	layer := m.GetLayerByName(name)
	if layer == nil {
		panic(fmt.Sprint("找不到层=", name))
	}

	return layer
}

func (m *Map) GetTileByFileName(name string) *TilesetTile {
	for _, tileset := range m.Tilesets {
		for _, tile := range tileset.Tiles {
			fileName := path.Base(tile.Image.Source)
			if fileName == name {
				return tile
			}
		}
	}

	return nil
}

func (m *Map) MustGetTileByFileName(name string) *TilesetTile {
	tile := m.GetTileByFileName(name)
	if tile == nil {
		panic(fmt.Sprint("找不到tile=", name))
	}

	return tile
}

func (border *Border) Contains(x int, y int) bool {
	containsX := border.MinX <= x && x <= border.MaxX
	containsY := border.MinY <= y && y <= border.MaxY
	return containsX && containsY
}
