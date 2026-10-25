// Package geo holds the Go type sqlc uses for PostGIS geography(Point, 4326) columns.
package geo

import (
	"encoding/hex"
	"fmt"

	"github.com/twpayne/go-geom/encoding/ewkb"
)

// Point wraps ewkb.Point. pgx returns an unknown type such as geography as hex-encoded EWKB text
// (a Go string), which ewkb.Point.Scan alone rejects ("want []byte, got string"); this type
// accepts the hex text form as well as raw EWKB bytes. X() is the longitude and Y() the latitude.
type Point struct {
	ewkb.Point
}

// Scan implements sql.Scanner.
func (p *Point) Scan(src interface{}) error {
	switch v := src.(type) {
	case string:
		return p.scanHex([]byte(v))
	case []byte:
		// Raw EWKB starts with the byte-order marker 0x00 or 0x01; hex text starts with '0'.
		if len(v) > 0 && v[0] == '0' {
			return p.scanHex(v)
		}
		return p.Point.Scan(v)
	default:
		return fmt.Errorf("geo.Point: cannot scan %T", src)
	}
}

func (p *Point) scanHex(h []byte) error {
	raw := make([]byte, hex.DecodedLen(len(h)))
	if _, err := hex.Decode(raw, h); err != nil {
		return fmt.Errorf("geo.Point: bad hex EWKB: %w", err)
	}
	return p.Point.Scan(raw)
}
