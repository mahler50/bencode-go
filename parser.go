package bencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

// impl this interface to hold data struct
// through the parsing time
type builder interface {
	// set value
	Int64(i int64)
	Uint64(u uint64)
	Float64(f float64)
	String(s string)
	Array()
	Map()

	// create sub-builders
	Elem(i int) builder
	Key(s string) builder

	// flush up to parent builder
	Flush()
}

func decodeInt64(r *bufio.Reader, delim byte) (data int64, err error) {
	buf, err := readSlice(r, delim)
	if err != nil {
		return
	}
	data, err = strconv.ParseInt(string(buf), 10, 64)
	return
}

// return bytes up until first delim without delim byte
func readSlice(r *bufio.Reader, delim byte) (data []byte, err error) {
	if data, err = r.ReadSlice(delim); err != nil {
		return
	}
	dataLen := len(data)
	if dataLen > 0 {
		data = data[:dataLen-1]
	} else {
		panic("bad r.ReadSlice() length")
	}

	return

}

func decodeString(r *bufio.Reader) (data string, err error) {
	length, err := decodeInt64(r, ':')
	if err != nil {
		return
	}
	if length < 0 {
		err = errors.New("bad string length")
		return
	}
	// try to get peek n length of bytes
	if peekBuf, peekErr := r.Peek(int(length)); peekErr == nil {
		data = string(peekBuf)
		_, err = r.Discard(int(length))
		return
	}

	buf := make([]byte, length)
	_, err = readFull(r, buf)
	if err != nil {
		return
	}
	data = string(buf)

	return
}

// bufio.Reader version of readFull
// copy from io.ReadFull
func readFull(r *bufio.Reader, buf []byte) (n int, err error) {
	return readAtLeast(r, buf, len(buf))
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

func parseFromReader(r *bufio.Reader, build builder) (err error) {
	c, err := r.ReadByte()
	if err != nil {
		goto exit
	}
	switch {
	case c >= '0' && c <= '9':
		// parse string
		err = r.UnreadByte()
		if err != nil {
			goto exit
		}
		var s string
		s, err = decodeString(r)
		if err != nil {
			goto exit
		}
		build.String(s)

	case c == 'i':
		var buf []byte
		buf, err = readSlice(r, 'e')
		if err != nil {
			goto exit
		}
		var str string
		var i int64
		var u uint64
		var f float64
		str = string(buf)
		if i, err = strconv.ParseInt(str, 10, 64); err == nil {
			build.Int64(i)
		} else if u, err = strconv.ParseUint(str, 10, 64); err == nil {
			build.Uint64(u)
		} else if f, err = strconv.ParseFloat(str, 64); err == nil {
			build.Float64(f)
		} else {
			err = errors.New("bad integer")
		}

	case c == 'd':
		build.Map()
		for {
			c, err = r.ReadByte()
			if err != nil {
				goto exit
			}
			if c == 'e' {
				break
			}
			err = r.UnreadByte()
			if err != nil {
				goto exit
			}
			var key string
			key, err = decodeString(r)
			if err != nil {
				goto exit
			}

			err = parseFromReader(r, build.Key(key))
			if err != nil {
				goto exit
			}
		}

	case c == 'l':
		build.Array()
		n := 0
		for {
			c, err = r.ReadByte()
			if err != nil {
				goto exit
			}
			if c == 'e' {
				break
			}
			err = r.UnreadByte()
			if err != nil {
				goto exit
			}
			err = parseFromReader(r, build.Elem(n))
			if err != nil {
				goto exit
			}
			n++
		}

	default:
		err = fmt.Errorf("unexpected character: %v", c)
	}
exit:
	build.Flush()
	return
}

func parse(reader io.Reader, builder builder) (err error) {
	r, ok := reader.(*bufio.Reader)
	if !ok {
		r = newBufioReader(reader)
		defer bufioReaderPool.Put(r)
	}

	return parseFromReader(r, builder)
}

// this is a pool to hold bufio.Reader
// using Get() to get a reader from the pool
// and using Put() to put it back
var bufioReaderPool sync.Pool

func newBufioReader(r io.Reader) *bufio.Reader {
	if v := bufioReaderPool.Get(); v != nil {
		br := v.(*bufio.Reader)
		br.Reset(r)
		return br
	}
	return bufio.NewReader(r)
}
