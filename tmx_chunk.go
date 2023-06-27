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

import "encoding/xml"

// LayerTile is a layer tile
type Chunk struct {
	X       int    `xml:"x,attr"`
	Y       int    `xml:"y,attr"`
	Width   int    `xml:"width,attr"`
	Height  int    `xml:"height,attr"`
	RawData []byte `xml:",innerxml"`

	Tiles     []*LayerTile
	data      *Data
	Layer     *Layer
	TileCount int
}

func (chunk *Chunk) decodeCSV() ([]uint32, error) {
	gids, err := chunk.data.decodeCSV()
	if err != nil {
		return []uint32{}, err
	}

	if len(gids) != chunk.Width*chunk.Height {
		return []uint32{}, ErrInvalidDecodedTileCount
	}

	return gids, nil
}

func (chunk *Chunk) decodeBase64() ([]uint32, error) {
	dataBytes, err := chunk.data.decodeBase64()
	if err != nil {
		return []uint32{}, err
	}

	if len(dataBytes) != chunk.Width*chunk.Height*4 {
		return []uint32{}, ErrInvalidDecodedTileCount
	}

	gids := make([]uint32, chunk.Width*chunk.Height)

	j := 0
	for y := 0; y < chunk.Height; y++ {
		for x := 0; x < chunk.Width; x++ {
			gid := uint32(dataBytes[j]) +
				uint32(dataBytes[j+1])<<8 +
				uint32(dataBytes[j+2])<<16 +
				uint32(dataBytes[j+3])<<24
			j += 4

			gids[y*chunk.Width+x] = gid
		}
	}

	return gids, nil
}

func (chunk *Chunk) decodeTiles() error {
	var gids []uint32
	var err error
	switch chunk.data.Encoding {
	case "csv":
		if gids, err = chunk.decodeCSV(); err != nil {
			return err
		}
	case "base64":
		if gids, err = chunk.decodeBase64(); err != nil {
			return err
		}
	default:
		return ErrUnknownEncoding
	}

	l := chunk.Layer

	tileCount := 0

	chunk.Tiles = make([]*LayerTile, len(gids))
	for j := 0; j < len(chunk.Tiles); j++ {
		tile, findError := l._map.TileGIDToTile(gids[j])
		if findError != nil {
			return findError
		}

		chunk.Tiles[j] = tile

		if tile.Nil {
			continue
		}

		tileCount++

		tile.XInChunk = j % chunk.Width
		tile.YInChunk = j / chunk.Width
		tile.X = chunk.X + tile.XInChunk
		tile.Y = chunk.Y + tile.YInChunk
	}

	chunk.TileCount = tileCount
	return nil
}

func (chunk *Chunk) DecodeChunk(layer *Layer) error {
	chunk.Layer = layer

	if chunk.RawData == nil {
		return ErrEmptyLayerData
	}

	chunk.data = &Data{
		Encoding:    layer.data.Encoding,
		Compression: layer.data.Compression,
		RawData:     chunk.RawData,
	}

	if err := chunk.decodeTiles(); err != nil {
		return err
	}

	// Data is not needed anymore
	chunk.data = nil
	return nil
}

func (chunk *Chunk) UnmarshalXML1(d *xml.Decoder, start xml.StartElement) error {
	item := aliasChunk{}

	if err := d.DecodeElement(&item, &start); err != nil {
		return err
	}

	*chunk = (Chunk)(item.internalChunk)
	chunk.data = item.Data
	return nil
}
