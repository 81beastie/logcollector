// Package wincp — декодирование вывода Windows-консольных утилит (wevtutil, reg):
// они пишут в OEM-кодировке (CP866 на русской локали), а не в UTF-8.
package wincp

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// Decode — превращает байты консольного вывода в UTF-8:
// валидный UTF-8 не трогается, остальное декодируется из CP866.
func Decode(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	out, err := charmap.CodePage866.NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "")
	}
	return string(out)
}
