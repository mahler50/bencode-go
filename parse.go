package bencodego

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
)

func decodeFromReader(r *bufio.Reader) (any, error) {
	result, err := unmarshal(r)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// it is a parser for bencode stream,
// the raw bencode could be converted
// into string, i64, slice[any] and
// map[string]any
func unmarshal(data *bufio.Reader) (any, error) {
	st, err := data.ReadByte()
	if err != nil {
		return nil, err
	}
	switch st {
	// handle i64
	case 'i':
		buf, err := optimisticReadBytes(data, 'e')
		if err != nil {
			return nil, err
		}
		buf = buf[:len(buf)-1]
		integer, err := strconv.Atoi(string(buf))
		if err != nil {
			return nil, err
		}

		return integer, nil
	// handle list
	case 'l':
		list := []any{}
		for {
			c, err := data.ReadByte()
			if err == nil {
				if c == 'e' {
					return list, nil
				} else {
					data.UnreadByte()
				}
			}
			value, err := unmarshal(data)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
	// handle dictionary
	case 'd':
		dic := map[string]any{}
		for {
			c, err := data.ReadByte()
			if err == nil {
				if c == 'e' {
					return dic, nil
				} else {
					data.UnreadByte()
				}
			}
			key, err := unmarshal(data)
			if err != nil {
				return nil, err
			}
			k, ok := key.(string)
			if !ok {
				return nil, errors.New("bencode: invalid string dictionary key")
			}

			value, err := unmarshal(data)
			if err != nil {
				return nil, err
			}

			dic[k] = value
		}
	// handle string
	default:
		data.UnreadByte()
		buf, err := optimisticReadBytes(data, ':')
		if err != nil {
			return nil, err
		}
		buf = buf[:len(buf)-1]
		length, err := strconv.ParseInt(string(buf), 10, 64)
		if err != nil {
			return nil, err
		}

		buffer := make([]byte, length)
		_, err = readAtLeast(data, buffer, int(length))

		return string(buffer), err
	}
}

func optimisticReadBytes(data *bufio.Reader, delim byte) (buffer []byte, err error) {
	buffered := data.Buffered()
	if buffer, err = data.Peek(buffered); err != nil {
		return nil, err
	}

	if i := bytes.IndexByte(buffer, delim); i >= 0 {
		return data.ReadSlice(delim)
	}
	return data.ReadBytes(delim)
}

// bufio.Reader version of readAtLeast
// copy from io.ReadAtLeast
func readAtLeast(r *bufio.Reader, buf []byte, min int) (n int, err error) {
	if len(buf) < min {
		return 0, io.ErrShortBuffer
	}

	for n < min && err == nil {
		var nn int
		nn, err = r.Read(buf[n:])
		n += nn
	}
	if n >= min {
		err = nil
	} else if n > 0 && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}
