package payload

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	Magic      = "V46LIFT1"
	FooterSize = 20
)

type Kind uint32

const (
	KindNone      Kind = 0
	KindLauncher  Kind = 1
	KindInstaller Kind = 2
)

func (k Kind) String() string {
	switch k {
	case KindNone:
		return "none"
	case KindLauncher:
		return "launcher"
	case KindInstaller:
		return "installer"
	default:
		return fmt.Sprintf("kind(%d)", uint32(k))
	}
}

type Info struct {
	Kind   Kind
	Offset int64
	Length int64
}

func Parse(data []byte) (kind Kind, payload []byte, stripped []byte, err error) {
	info, err := InspectBytes(data)
	if err != nil {
		return KindNone, nil, nil, err
	}
	if info.Kind == KindNone {
		return KindNone, nil, data, nil
	}
	return info.Kind, data[info.Offset : info.Offset+info.Length], data[:info.Offset], nil
}

func InspectBytes(data []byte) (Info, error) {
	if len(data) < FooterSize {
		return Info{}, nil
	}
	return inspectFooter(data[len(data)-FooterSize:], int64(len(data)))
}

func InspectFile(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return Info{}, err
	}
	if st.Size() < FooterSize {
		return Info{}, nil
	}

	var footer [FooterSize]byte
	if _, err := f.ReadAt(footer[:], st.Size()-FooterSize); err != nil {
		return Info{}, err
	}
	return inspectFooter(footer[:], st.Size())
}

func inspectFooter(footer []byte, size int64) (Info, error) {
	if string(footer[12:20]) != Magic {
		return Info{}, nil
	}
	n := binary.LittleEndian.Uint64(footer[0:8])
	kind := Kind(binary.LittleEndian.Uint32(footer[8:12]))
	if kind == KindNone {
		return Info{}, fmt.Errorf("invalid packed payload kind")
	}
	if n > uint64(size-FooterSize) {
		return Info{}, fmt.Errorf("truncated packed payload")
	}
	return Info{
		Kind:   kind,
		Offset: size - FooterSize - int64(n),
		Length: int64(n),
	}, nil
}

func Strip(data []byte) []byte {
	kind, _, stripped, err := Parse(data)
	if err != nil || kind == KindNone {
		return data
	}
	return stripped
}

func Append(base []byte, kind Kind, payload []byte) []byte {
	base = Strip(base)
	out := make([]byte, 0, len(base)+len(payload)+FooterSize)
	out = append(out, base...)
	out = append(out, payload...)
	out = appendFooter(out, kind, uint64(len(payload)))
	return out
}

func WriteFooter(w io.Writer, kind Kind, payloadLen uint64) error {
	var footer [FooterSize]byte
	putFooter(&footer, kind, payloadLen)
	_, err := w.Write(footer[:])
	return err
}

func appendFooter(out []byte, kind Kind, payloadLen uint64) []byte {
	var footer [FooterSize]byte
	putFooter(&footer, kind, payloadLen)
	return append(out, footer[:]...)
}

func putFooter(footer *[FooterSize]byte, kind Kind, payloadLen uint64) {
	binary.LittleEndian.PutUint64(footer[0:8], payloadLen)
	binary.LittleEndian.PutUint32(footer[8:12], uint32(kind))
	copy(footer[12:20], Magic)
}

func Read(path string) (Kind, []byte, error) {
	info, err := InspectFile(path)
	if err != nil {
		return KindNone, nil, err
	}
	if info.Kind == KindNone {
		return KindNone, nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return KindNone, nil, err
	}
	defer f.Close()

	buf := make([]byte, info.Length)
	if _, err := f.ReadAt(buf, info.Offset); err != nil {
		return KindNone, nil, err
	}
	return info.Kind, buf, nil
}

func OpenSection(path string) (Info, io.ReaderAt, *os.File, error) {
	info, err := InspectFile(path)
	if err != nil {
		return Info{}, nil, nil, err
	}
	if info.Kind == KindNone {
		return Info{}, nil, nil, fmt.Errorf("not a packed v46lift binary")
	}
	f, err := os.Open(path)
	if err != nil {
		return Info{}, nil, nil, err
	}
	return info, io.NewSectionReader(f, info.Offset, info.Length), f, nil
}
