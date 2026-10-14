package wallet

import "fmt"

// FormatRupiah formats an integer amount to Indonesian Rupiah currency string (e.g. 50000 -> "50.000")
func FormatRupiah(n int64) string {
	return formatRupiah(n)
}

func formatRupiah(n int64) string {
	in := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(in)+len(in)/3)
	for i, c := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
