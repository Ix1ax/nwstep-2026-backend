package prng

// SplitMix64 — детерминированный генератор псевдослучайных чисел согласно разделу 9 ТЗ.
// Обеспечивает строгую воспроизводимость результатов вне зависимости от платформы и времени.
type SplitMix64 struct {
	state uint64
}

// Константы масок для инициализации независимых потоков псевдослучайных чисел
const (
	StreamEnvironmentMask uint64 = 0xA0761D6478BD642F // Поток шума среды
	StreamMutationsMask   uint64 = 0xE7037ED1A0B428DB // Поток мутаций генома
	StreamPlacementMask   uint64 = 0x8EBC6AF09C88C6E3 // Поток геометрии размещения потомков

	Increment uint64 = 0x9E3779B97F4A7C15
)

// NewSplitMix64 создает новый генератор с начальным состоянием
func NewSplitMix64(state uint64) *SplitMix64 {
	return &SplitMix64{state: state}
}

// NextUint64 возвращает следующее 64-битное псевдослучайное число.
// Тестовый вектор: при state = 0 первый результат равен 0xE220A8397B1DCDAF.
func (sm *SplitMix64) NextUint64() uint64 {
	sm.state += Increment
	z := (sm.state ^ (sm.state >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// NextFloat64 возвращает вещественное число в диапазоне [0, 1)
func (sm *SplitMix64) NextFloat64() float64 {
	return float64(sm.NextUint64()>>11) / float64(uint64(1)<<53)
}

// State возвращает текущее состояние генератора
func (sm *SplitMix64) State() uint64 {
	return sm.state
}

// Streams инкапсулирует три независимых потока PRNG для изоляции шума среды от мутаций
type Streams struct {
	Environment *SplitMix64 // Шум источников
	Mutations   *SplitMix64 // Мутации при делении
	Placement   *SplitMix64 // Поворот при размещении потомков
}

// NewStreams инициализирует 3 изолированных потока на основе одного базового seed
func NewStreams(seed uint64) *Streams {
	return &Streams{
		Environment: NewSplitMix64(seed ^ StreamEnvironmentMask),
		Mutations:   NewSplitMix64(seed ^ StreamMutationsMask),
		Placement:   NewSplitMix64(seed ^ StreamPlacementMask),
	}
}
