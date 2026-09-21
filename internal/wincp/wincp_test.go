package wincp

import "testing"

func TestDecode_CP866ErrorBecomesReadable(t *testing.T) {
	// "Ошибка" в CP866: О=0x8E, ш=0x98, и=0x88, б=0xA1, к=0x8A, а=0xA0
	b := []byte{0x8E, 0xE8, 0xA8, 0xA1, 0xAA, 0xA0}
	if got := Decode(b); got != "Ошибка" {
		t.Errorf("Decode = %q, ожидала %q", got, "Ошибка")
	}
}

func TestDecode_ValidUTF8Untouched(t *testing.T) {
	if got := Decode([]byte("Доступ запрещён")); got != "Доступ запрещён" {
		t.Errorf("валидный UTF-8 искажён: %q", got)
	}
}

func TestDecode_MixedASCIIAndCP866(t *testing.T) {
	// "wevtutil: Доступ" — ASCII + CP866
	b := append([]byte("wevtutil: "), 0x84, 0xAE, 0xE1, 0xE2, 0xE3, 0xAF)
	want := "wevtutil: Доступ"
	if got := Decode(b); got != want {
		t.Errorf("Decode = %q, ожидала %q", got, want)
	}
}

func TestDecode_Empty(t *testing.T) {
	if got := Decode(nil); got != "" {
		t.Errorf("пустой ввод: %q", got)
	}
}
