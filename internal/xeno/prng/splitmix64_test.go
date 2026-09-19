package prng_test

import (
	"testing"

	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/prng"
)

func TestSplitMix64_Determinism(t *testing.T) {
	// Согласно ТЗ (раздел 6.1) эталонный вектор:
	// SplitMix64(0) выдает 0xE220A8397B1DCDAF на первом шаге
	p := prng.NewSplitMix64(0)
	val := p.NextUint64()
	expected := uint64(0xE220A8397B1DCDAF)
	if val != expected {
		t.Fatalf("SplitMix64(0) = 0x%X, expected 0x%X", val, expected)
	}

	// Проверка независимости изолированных потоков
	streams := prng.NewStreams(42)
	env1 := streams.Environment.NextFloat64()
	mut1 := streams.Mutations.NextFloat64()
	place1 := streams.Placement.NextFloat64()

	// Пересоздаем с тем же seed 42
	streams2 := prng.NewStreams(42)
	if streams2.Environment.NextFloat64() != env1 {
		t.Fatalf("Environment stream is not deterministic")
	}
	if streams2.Mutations.NextFloat64() != mut1 {
		t.Fatalf("Mutations stream is not deterministic")
	}
	if streams2.Placement.NextFloat64() != place1 {
		t.Fatalf("Placement stream is not deterministic")
	}
}
