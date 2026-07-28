package file

import (
	"archive/tar"
	"os"
)

const (
	TypeRegular Type = iota
	TypeHardLink
	TypeSymLink
	TypeCharacterDevice
	TypeBlockDevice
	TypeDirectory
	TypeFIFO
	TypeSocket
	TypeIrregular
)

// why use a rune type? we're looking for something that is memory compact but is easily human interpretable.

type Type int

func AllTypes() []Type {
	return []Type{
		TypeRegular,
		TypeHardLink,
		TypeSymLink,
		TypeCharacterDevice,
		TypeBlockDevice,
		TypeDirectory,
		TypeFIFO,
		TypeSocket,
		TypeIrregular,
	}
}

func TypeFromTarType(ty byte) Type {
	switch ty {
	case tar.TypeReg, tar.TypeRegA: //nolint: staticcheck
		return TypeRegular
	case tar.TypeLink:
		return TypeHardLink
	case tar.TypeSymlink:
		return TypeSymLink
	case tar.TypeChar:
		return TypeCharacterDevice
	case tar.TypeBlock:
		return TypeBlockDevice
	case tar.TypeDir:
		return TypeDirectory
	case tar.TypeFifo:
		return TypeFIFO
	default:
		return TypeIrregular
	}
}

func TypeFromMode(mode os.FileMode) Type {
	switch {
	case isSet(mode, os.ModeSymlink):
		return TypeSymLink
	case isSet(mode, os.ModeIrregular):
		return TypeIrregular
	case isSet(mode, os.ModeCharDevice):
		return TypeCharacterDevice
	case isSet(mode, os.ModeDevice):
		return TypeBlockDevice
	case isSet(mode, os.ModeNamedPipe):
		return TypeFIFO
	case isSet(mode, os.ModeSocket):
		return TypeSocket
	case mode.IsDir():
		return TypeDirectory
	case mode.IsRegular():
		return TypeRegular
	default:
		return TypeIrregular
	}
}

func isSet(mode, field os.FileMode) bool {
	return mode&field != 0
}

func (t Type) String() string {
	switch t {
	case TypeRegular:
		return "RegularFile"
	case TypeHardLink:
		return "HardLink"
	case TypeSymLink:
		return "SymbolicLink"
	case TypeCharacterDevice:
		return "CharacterDevice"
	case TypeBlockDevice:
		return "BlockDevice"
	case TypeDirectory:
		return "Directory"
	case TypeFIFO:
		return "FIFONode"
	case TypeSocket:
		return "Socket"
	case TypeIrregular:
		return "IrregularFile"
	default:
		return "Unknown"
	}
}
