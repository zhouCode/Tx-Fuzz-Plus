package main

import (
	"fmt"

	"github.com/MariusVanDerWijden/tx-fuzz/helper"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/program"
)

func main() {
	fmt.Println("selfdestruct")
	if err := helper.Execute(Selfdestructor(), 5_000_000); err != nil {
		panic(err)
	}
	fmt.Println("ef-byte")
	if err := helper.Execute(EfByte(), 5_000_000); err != nil {
		panic(err)
	}
}

func Selfdestructor() []byte {
	selfdestructTo := []byte{
		byte(vm.PUSH1),
		0,
		byte(vm.CALLDATALOAD),
		byte(vm.PUSH20),
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		byte(vm.AND),
		byte(vm.SELFDESTRUCT),
	}

	initcode := program.New()
	initcode.Mstore(selfdestructTo, 0)
	initcode.Return(0, len(selfdestructTo))

	program := program.New()
	Create(program, selfdestructTo, false, true)
	program.Op(vm.POP)
	Create(program, selfdestructTo, true, false)
	program.Op(vm.POP)
	Create(program, initcode.Bytes(), true, false)
	return program.Bytes()
}

func EfByte() []byte {
	inner := []byte{0xEF}

	initcode := program.New()
	initcode.Mstore(inner, 0)
	initcode.Return(0, len(inner))

	program := program.New()
	Create(program, initcode.Bytes(), false, false)
	program.Op(vm.POP)
	Create(program, initcode.Bytes(), true, true)
	program.Op(vm.POP)
	return program.Bytes()
}

func Create(p *program.Program, code []byte, inMemory bool, isCreate2 bool) {
	var (
		value    = 0
		offset   = 0
		size     = len(code)
		salt     = 0
		createOp = vm.CREATE
	)
	if !inMemory {
		p.Mstore(code, 0)
	}
	if isCreate2 {
		p.Push(salt)
		createOp = vm.CREATE2
	}
	p.Push(size).Push(offset).Push(value).Op(createOp)
}
