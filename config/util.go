package config

import (
	"encoding"
	"time"

	"github.com/filecoin-project/go-address"
)

// Duration is a wrapper type for Duration
// for decoding and encoding from/to TOML
type Duration time.Duration

// UnmarshalText implements interface for TOML decoding
func (dur *Duration) UnmarshalText(text []byte) error {
	d, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*dur = Duration(d)
	return err
}

func (dur Duration) MarshalText() ([]byte, error) {
	d := time.Duration(dur)
	return []byte(d.String()), nil
}

var (
	_ encoding.TextMarshaler   = (*Duration)(nil)
	_ encoding.TextUnmarshaler = (*Duration)(nil)
)

// Address is a wrapper type for Address
// for decoding and encoding from/to TOML
type Address address.Address

// UnmarshalText implements interface for TOML decoding
func (addr *Address) UnmarshalText(text []byte) error {
	d, err := address.NewFromString(string(text))
	if err != nil {
		return err
	}
	*addr = Address(d)
	return err
}

func (addr Address) MarshalText() ([]byte, error) {
	if address.Address(addr) == address.Undef {
		return []byte{}, nil
	}
	return []byte(address.Address(addr).String()), nil
}

func (addr Address) Unwrap() address.Address {
	return address.Address(addr)
}

func CfgAddrArrToNative(addrs []Address) []address.Address {
	nativeAddrs := make([]address.Address, len(addrs))
	for _, addr := range addrs {
		nativeAddrs = append(nativeAddrs, address.Address(addr))
	}
	return nativeAddrs
}
