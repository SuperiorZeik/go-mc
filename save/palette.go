package save

import (
	"io"
	"strconv"

	pk "github.com/Tnze/go-mc/net/packet"
)

type BlockState interface {
}

type PaletteContainer struct {
	maps   blockMaps
	config func(p *PaletteContainer, bits byte)
	palette
	BitStorage
}

func (p *PaletteContainer) ReadFrom(r io.Reader) (n int64, err error) {
	var bits pk.UnsignedByte
	n, err = bits.ReadFrom(r)
	if err != nil {
		return
	}
	p.config(p, byte(bits))

	nn, err := p.palette.ReadFrom(r)
	n += nn
	if err != nil {
		return n, err
	}

	nn, err = p.BitStorage.ReadFrom(r)
	n += nn
	if err != nil {
		return n, err
	}
	return n, nil
}

func createStatesPalette(p *PaletteContainer, bits byte) {
	switch bits {
	case 0:
		p.palette = &singleValuePalette{
			onResize: nil,
			maps:     p.maps,
			v:        nil,
		}
	case 1, 2, 3, 4:
		p.palette = &linearPalette{
			onResize: nil,
			maps:     p.maps,
			bits:     4,
		}
	case 5, 6, 7, 8:
		// TODO: HashMapPalette
		p.palette = &linearPalette{
			onResize: nil,
			maps:     p.maps,
			bits:     4,
		}
	default:
		p.palette = &globalPalette{
			maps: p.maps,
		}
	}
}

func createBiomesPalette(p *PaletteContainer, bits byte) {
	switch bits {
	case 0:
		p.palette = &singleValuePalette{
			onResize: nil,
			maps:     p.maps,
			v:        nil,
		}
	case 1, 2, 3:
		p.palette = &linearPalette{
			onResize: nil,
			maps:     p.maps,
			bits:     4,
		}
	default:
		p.palette = &globalPalette{
			maps: p.maps,
		}
	}
}

func (p *PaletteContainer) WriteTo(w io.Writer) (n int64, err error) {
	return pk.Tuple{
		pk.UnsignedByte(p.bits),
		p.palette,
		p.BitStorage,
	}.WriteTo(w)
}

type palette interface {
	pk.FieldEncoder
	pk.FieldDecoder
	id(v BlockState) int
	value(i int) BlockState
}

type blockMaps interface {
	getID(state BlockState) (id int)
	getValue(id int) (state BlockState)
}

type singleValuePalette struct {
	onResize func(n int, v BlockState) int
	maps     blockMaps
	v        BlockState
}

func (s *singleValuePalette) id(v BlockState) int {
	if s.v == nil {
		s.v = v
		return 0
	}
	if s.v == v {
		return 0
	}
	// We have 2 values now. At least 1 bit is required.
	return s.onResize(1, v)
}

func (s *singleValuePalette) value(i int) BlockState {
	if s.v != nil && i == 0 {
		return s.v
	}
	panic("singleValuePalette: " + strconv.Itoa(i) + " out of bounds")
}

func (s *singleValuePalette) ReadFrom(r io.Reader) (n int64, err error) {
	var i pk.VarInt
	n, err = i.ReadFrom(r)
	if err != nil {
		return
	}
	s.v = s.maps.getValue(int(i))
	return
}

func (s *singleValuePalette) WriteTo(w io.Writer) (n int64, err error) {
	return pk.VarInt(s.maps.getID(s.v)).WriteTo(w)
}

type linearPalette struct {
	onResize func(n int, v BlockState) int
	maps     blockMaps
	values   []BlockState
	bits     int
}

func (l *linearPalette) id(v BlockState) int {
	for i, t := range l.values {
		if t == v {
			return i
		}
	}
	if cap(l.values)-len(l.values) > 0 {
		l.values = append(l.values, v)
		return len(l.values) - 1
	}
	return l.onResize(l.bits+1, v)
}

func (l *linearPalette) value(i int) BlockState {
	if i >= 0 && i < len(l.values) {
		return l.values[i]
	}
	return nil
}

func (l *linearPalette) ReadFrom(r io.Reader) (n int64, err error) {
	var size, value pk.VarInt
	if n, err = size.ReadFrom(r); err != nil {
		return
	}
	for i := 0; i < int(size); i++ {
		if nn, err := value.ReadFrom(r); err != nil {
			return n + nn, err
		} else {
			n += nn
		}
		l.values[i] = l.maps.getValue(int(value))
	}
	return
}

func (l *linearPalette) WriteTo(w io.Writer) (n int64, err error) {
	if n, err = pk.VarInt(len(l.values)).WriteTo(w); err != nil {
		return
	}
	for _, v := range l.values {
		if nn, err := pk.VarInt(l.maps.getID(v)).WriteTo(w); err != nil {
			return n + nn, err
		} else {
			n += nn
		}
	}
	return
}

type globalPalette struct {
	maps blockMaps
}

func (g *globalPalette) id(v BlockState) int {
	return g.maps.getID(v)
}

func (g *globalPalette) value(i int) BlockState {
	return g.value(i)
}

func (g *globalPalette) ReadFrom(_ io.Reader) (int64, error) {
	return 0, nil
}

func (g *globalPalette) WriteTo(_ io.Writer) (int64, error) {
	return 0, nil
}
