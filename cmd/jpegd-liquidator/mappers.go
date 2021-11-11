package main

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/alecthomas/kong"
)

func GetMappers() []kong.Option {
	mappers := make([]kong.Option, 1)

	mappers[0] = kong.NamedMapper("BigFloat", new(BigFloatMapper))

	return mappers
}

type BigFloatMapper func(ctx *kong.DecodeContext, target reflect.Value) error

func (m *BigFloatMapper) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	res := ctx.Scan.Pop().String()

	num, ok := new(big.Float).SetString(res)
	if !ok {
		return errors.New("error while parsing big float")
	}
	target.Set(reflect.ValueOf(num))

	return nil
}
