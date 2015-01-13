package ioutil

import (
		"io"
		"encoding/binary"
)

func ReadFramed(reader io.Reader) ([]byte, error) {
	var size uint16
	err := binary.Read(reader, binary.BigEndian, &size)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return []byte{}, nil
	}
	buffer := make([]byte, size, size)
	_, err = io.ReadFull(reader, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}
