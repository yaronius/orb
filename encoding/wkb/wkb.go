// Package wkb is for decoding ESRI's Well Known Binary (WKB) format
// sepcification at http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm
package wkb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/paulmach/orb"
)

const (
	pointType              uint32 = 1
	lineStringType         uint32 = 2
	polygonType            uint32 = 3
	multiPointType         uint32 = 4
	multiLineStringType    uint32 = 5
	multiPolygonType       uint32 = 6
	geometryCollectionType uint32 = 7
)

// DefaultByteOrder is the order used form marshalling or encoding
// is none is specified.
var DefaultByteOrder binary.ByteOrder = binary.LittleEndian

// An Encoder will encode a geometry as WKB to the writer given at
// creation time.
type Encoder struct {
	buf []byte

	w     io.Writer
	order binary.ByteOrder
}

// Marshal encodes the geometry with the given byte order.
func Marshal(geom orb.Geometry, byteOrder ...binary.ByteOrder) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	e := NewEncoder(buf)
	if len(byteOrder) > 0 {
		e.SetByteOrder(byteOrder[0])
	}

	err := e.Encode(geom)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// NewEncoder creates a new Encoder for the given writer
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:     w,
		order: DefaultByteOrder,
	}
}

// SetByteOrder will override the default byte order set when
// the encoder was created.
func (e *Encoder) SetByteOrder(bo binary.ByteOrder) {
	e.order = bo
}

// Encode will write the geometry encoded as WKB to the given writer.
func (e *Encoder) Encode(geom orb.Geometry) error {
	if geom == nil {
		return nil
	}

	// deal with types that are not supported by wkb
	switch g := geom.(type) {
	case orb.Ring:
		geom = orb.Polygon{g}
	case orb.Bound:
		geom = g.ToPolygon()
	}

	var b []byte
	if e.order == binary.LittleEndian {
		b = []byte{1}
	} else {
		b = []byte{0}
	}

	_, err := e.w.Write(b)
	if err != nil {
		return err
	}

	if e.buf == nil {
		e.buf = make([]byte, 16)
	}

	switch g := geom.(type) {
	case orb.Point:
		return e.writePoint(g)
	case orb.MultiPoint:
		return e.writeMultiPoint(g)
	case orb.LineString:
		return e.writeLineString(g)
	case orb.MultiLineString:
		return e.writeMultiLineString(g)
	case orb.Polygon:
		return e.writePolygon(g)
	case orb.MultiPolygon:
		return e.writeMultiPolygon(g)
	case orb.Collection:
		return e.writeCollection(g)
	}

	panic("unsupported type")
}

// Decoder can decoder WKB geometry off of the stream.
type Decoder struct {
	r io.Reader
}

// Unmarshal will decode the type into a Geometry.
func Unmarshal(b []byte) (orb.Geometry, error) {
	return NewDecoder(bytes.NewReader(b)).Decode()
}

// NewDecoder will create a new WKB decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r: r,
	}
}

// Decode will decode the next geometry off of the steam.
func (d *Decoder) Decode() (orb.Geometry, error) {
	byteOrder, typ, err := readByteOrderType(d.r)
	if err != nil {
		return nil, err
	}

	switch typ {
	case pointType:
		return readPoint(d.r, byteOrder)
	case multiPointType:
		return readMultiPoint(d.r, byteOrder)
	case lineStringType:
		return readLineString(d.r, byteOrder)
	case multiLineStringType:
		return readMultiLineString(d.r, byteOrder)
	case polygonType:
		return readPolygon(d.r, byteOrder)
	case multiPolygonType:
		return readMultiPolygon(d.r, byteOrder)
	case geometryCollectionType:
		return readCollection(d.r, byteOrder)
	}

	return nil, fmt.Errorf("unknown geometry: %v", typ)
}

func readByteOrderType(r io.Reader) (binary.ByteOrder, uint32, error) {
	var bom = make([]byte, 1)

	// the bom is the first byte
	if _, err := r.Read(bom); err != nil {
		return nil, 0, err
	}

	var byteOrder binary.ByteOrder
	if bom[0] == 0 {
		byteOrder = binary.BigEndian
	} else {
		byteOrder = binary.LittleEndian
	}

	// the type which is 4 bytes
	var typ uint32

	err := binary.Read(r, byteOrder, &typ)
	if err != nil {
		return nil, 0, err
	}

	return byteOrder, typ, err
}