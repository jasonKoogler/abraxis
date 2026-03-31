package uuid

import (
	"github.com/segmentio/ksuid"
)

type UUID struct {
	ksuid.KSUID
}

func New() (UUID, error) {
	ksuid, err := ksuid.NewRandom()
	if err != nil {
		return UUID{}, err
	}
	return UUID{ksuid}, nil
}

func Parse(value string) (UUID, error) {
	ksuid, err := ksuid.Parse(value)
	if err != nil {
		return UUID{}, err
	}
	return UUID{ksuid}, nil
}

func Nil() UUID {
	return UUID{ksuid.Nil}
}
