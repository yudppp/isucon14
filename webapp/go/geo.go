package main

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
)

type GeoPoint struct {
	Lat int
	Lon int
}

func (g *GeoPoint) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("GeoPoint: cannot scan type %T", src)
	}
	if len(b) < 21 {
		return fmt.Errorf("GeoPoint: invalid WKB POINT length %d", len(b))
	}

	byteOrder := b[0]
	var order binary.ByteOrder
	if byteOrder == 1 {
		order = binary.LittleEndian
	} else {
		order = binary.BigEndian
	}

	wkbType := order.Uint32(b[1:5])
	if wkbType != 1 {
		return fmt.Errorf("GeoPoint: not a POINT WKB type")
	}

	x := math.Float64frombits(order.Uint64(b[5:13]))
	y := math.Float64frombits(order.Uint64(b[13:21]))

	// MySQLのPOINTは (X=経度, Y=緯度) 順
	// float64 から int へキャスト（小数点以下切り捨て）
	g.Lon = int(x)
	g.Lat = int(y)

	return nil
}

func (g GeoPoint) Value() (driver.Value, error) {
	// int を float64 に変換してWKBを生成
	buf := make([]byte, 21)
	buf[0] = 1 // little endian
	binary.LittleEndian.PutUint32(buf[1:5], 1)
	binary.LittleEndian.PutUint64(buf[5:13], math.Float64bits(float64(g.Lon)))
	binary.LittleEndian.PutUint64(buf[13:21], math.Float64bits(float64(g.Lat)))
	return buf, nil
}
