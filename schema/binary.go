package ui

import (
	"encoding/binary"
	"unicode/utf8"
)

//go:generate go run ../internal/uibinarygen

const BinaryVersion = 1
const uiHeaderBytes = 16

func decodeBinary(data []byte) (Template, error) {
	if len(data) < uiHeaderBytes || len(data) > MaxAssetBytes || string(data[:4]) != "KUI1" ||
		binary.LittleEndian.Uint16(data[4:6]) != BinaryVersion || binary.LittleEndian.Uint16(data[6:8]) != 0 ||
		binary.LittleEndian.Uint32(data[8:12]) != uint32(len(data)-uiHeaderBytes) || binary.LittleEndian.Uint32(data[12:16]) != 0 {
		return Template{}, ErrTemplate
	}

	r := uiReader{data: data[uiHeaderBytes:]}

	value := r.readTemplate()
	if r.invalid || r.position != len(r.data) {
		return Template{}, ErrTemplate
	}

	return value, nil
}

func encodeBinary(value Template) ([]byte, error) {
	writer := uiWriter{data: make([]byte, uiHeaderBytes)}
	writer.writeTemplate(value)

	if writer.invalid {
		return nil, ErrTemplate
	}

	copy(writer.data, "KUI1")
	binary.LittleEndian.PutUint16(writer.data[4:6], BinaryVersion)
	binary.LittleEndian.PutUint32(writer.data[8:12], uint32(len(writer.data)-uiHeaderBytes))

	return writer.data, nil
}

type uiWriter struct {
	data    []byte
	invalid bool
}

func (w *uiWriter) reserve(size int) []byte {
	if w.invalid || size < 0 || size > MaxAssetBytes-len(w.data) {
		w.invalid = true

		return nil
	}

	start := len(w.data)
	w.data = append(w.data, make([]byte, size)...)

	return w.data[start:]
}
func (w *uiWriter) writeU8(v uint8) {
	if data := w.reserve(1); data != nil {
		data[0] = v
	}
}
func (w *uiWriter) writeU16(v uint16) {
	if data := w.reserve(2); data != nil {
		binary.LittleEndian.PutUint16(data, v)
	}
}
func (w *uiWriter) writeU32(v uint32) {
	if data := w.reserve(4); data != nil {
		binary.LittleEndian.PutUint32(data, v)
	}
}
func (w *uiWriter) writeI32(v int32) { w.writeU32(uint32(v)) }
func (w *uiWriter) writeBool(v bool) {
	if v {
		w.writeU8(1)
	} else {
		w.writeU8(0)
	}
}
func (w *uiWriter) writeString(value string) {
	if len(value) > MaxAssetBytes || !utf8.ValidString(value) {
		w.invalid = true

		return
	}

	w.writeU32(uint32(len(value)))
	copy(w.reserve(len(value)), value)
}
func (w *uiWriter) writeElements(v []Element) {
	w.writeU32(uint32(len(v)))

	for _, value := range v {
		w.writeElement(value)
	}
}

type uiReader struct {
	data     []byte
	position int
	invalid  bool
}

func (r *uiReader) take(size int) []byte {
	if r.invalid || size < 0 || size > len(r.data)-r.position {
		r.invalid = true

		return nil
	}

	data := r.data[r.position : r.position+size]
	r.position += size

	return data
}
func (r *uiReader) readU8() uint8 {
	data := r.take(1)
	if data == nil {
		return 0
	}

	return data[0]
}
func (r *uiReader) readU16() uint16 {
	data := r.take(2)
	if data == nil {
		return 0
	}

	return binary.LittleEndian.Uint16(data)
}
func (r *uiReader) readU32() uint32 {
	data := r.take(4)
	if data == nil {
		return 0
	}

	return binary.LittleEndian.Uint32(data)
}
func (r *uiReader) readI32() int32 { return int32(r.readU32()) }
func (r *uiReader) readBool() bool {
	value := r.readU8()
	if value > 1 {
		r.invalid = true
	}

	return value == 1
}
func (r *uiReader) readString() string {
	size := r.readU32()
	if uint64(size) > uint64(len(r.data)-r.position) {
		r.invalid = true

		return ""
	}

	data := r.take(int(size))
	if !utf8.Valid(data) {
		r.invalid = true

		return ""
	}

	return string(data)
}
func (r *uiReader) readElements() []Element {
	count := r.readU32()
	if r.invalid || count == 0 || count > MaxElements || uint64(count) > uint64(len(r.data)-r.position) {
		r.invalid = true

		return nil
	}

	values := make([]Element, int(count))

	for index := range values {
		values[index] = r.readElement()
		if r.invalid {
			return nil
		}
	}

	return values
}
