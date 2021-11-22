package main

import (
	"testing"
)

func TestExtractCode(t *testing.T) {
	gift, _ := extractNitroCode("discord.gift/23t23ofuhi23fou24f")
	if gift != "23t23ofuhi23fou24f" {
		t.Errorf("not equal: %s", gift)
	}
}

//0.0000048 ns/op
//0.0000007 ns/op
func BenchmarkExtractCode(b *testing.B) {
	gift, _ := extractNitroCode("discord.gift/23t23ofuhi23fou24f")
	if gift != "23t23ofuhi23fou24f" {
		b.Errorf("not equal: %s", gift)
	}
}

//0.0000012 ns/op
func BenchmarkExtractCodeSplit(b *testing.B) {
	gift := extraNitroSplit("discord.gift/23t23ofuhi23fou24f/")
	if gift != "23t23ofuhi23fou24f" {
		b.Errorf("not equal: %s", gift)
	}
}

//0.0000048 ns/op
func BenchmarkExtractCodeRegex(b *testing.B) {
	gift := extractNitroCodeRegex("discord.gift/23t23ofuhi23fou24f/")
	if gift[0] != "23t23ofuhi23fou24f" {
		b.Errorf("not equal: %s", gift)
	}
}
